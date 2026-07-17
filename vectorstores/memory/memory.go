/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 21:29:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-17 09:01:56
 * @FilePath: \go-llmx\vectorstores\memory\memory.go
 * @Description: 内存向量库 —— 进程内余弦相似度检索（单测/原型/小规模数据）.
 * 检索路径：写入时预归一化缓存 + 查询 dot-only + Top-K 最小堆 + 并发分片；
 * 相似度数学见 math.go，生产规模见 redis/pgvector 适配器
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcmemory

import (
	"container/heap"
	"context"
	"math"
	"runtime"
	"sort"
	"sync"

	llmx "github.com/kamalyes/go-llmx"
)

// entry 单条存储记录（文档 + 向量配对）.
// [EN] A stored record (document + vector pair).
type entry struct {
	// doc 检索文档.
	// [EN] The document.
	doc llmx.Document

	// vector 嵌入向量（原始）.
	// [EN] The embedding vector (raw).
	vector []float64

	// unit 归一化副本（预计算缓存：余弦检索只看方向，
	// 写入后永不改变，摊销到每次检索省 2/3 乘加）.
	// [EN] Normalized copy (precomputed: cosine cares about direction only).
	unit []float64
}

// Store 内存向量库.
// [EN] In-memory vector store.
type Store struct {
	mu      sync.RWMutex
	entries []entry
}

// New 构造内存向量库.
// [EN] Build an in-memory store.
func New() *Store {
	return &Store{}
}

// AddDocuments 实现 llmx.VectorStore（写入文档与向量，数量需一致且向量非空）.
// [EN] Implement llmx.VectorStore (docs and vectors must align and be non-empty).
func (s *Store) AddDocuments(_ context.Context, docs []llmx.Document, vectors [][]float64) error {
	if len(docs) != len(vectors) {
		return llmx.ErrInvalidVectors
	}
	for _, v := range vectors {
		if len(v) < MinVectorDim {
			return llmx.ErrInvalidVectors
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, make([]entry, len(docs))...)
	tail := len(s.entries) - len(docs)
	for i, d := range docs {
		vec := append([]float64(nil), vectors[i]...)
		s.entries[tail+i] = entry{doc: d, vector: vec, unit: normalizeCopy(vec)}
	}
	return nil
}

// SimilaritySearch 实现 llmx.VectorStore（余弦相似度 Top-K，filters 可选）.
//
// 优化路径：查询向量归一化一次后每条只需点积（存量范数已预计算），
// Top-K 用 size=k 最小堆替代全量排序，同分按写入序稳定；
// 条数超过 ShardThreshold 时分片并行扫描后合并.
// [EN] Implement llmx.VectorStore (cosine top-K, filters optional).
// Optimized: query normalized once, per-entry dot product only,
// size-k min-heap instead of full sort; sharded when large.
func (s *Store) SimilaritySearch(_ context.Context, query []float64, topK int, filters ...llmx.Filter) ([]llmx.Document, error) {
	if len(query) < MinVectorDim {
		return nil, llmx.ErrInvalidVectors
	}
	if topK <= 0 {
		topK = DefaultTopK
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	q := normalizeCopy(query)
	if n := len(s.entries); n > ShardThreshold {
		return searchSharded(s.entries, q, topK, filters), nil
	}
	return searchSerial(s.entries, q, topK, filters), nil
}

// Len 返回当前存储条数.
// [EN] Return the number of stored entries.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// hit 堆元素（分数 + 写入序号用于同分稳定）.
// [EN] Heap element (score + insertion index for stability).
type hit struct {
	score float64
	idx   int
	doc   llmx.Document
}

// hitHeap size ≤ topK 的最小堆：堆顶是最差元素（同分时序号大的更差）.
// [EN] Min-heap of size ≤ topK: the top is the worst element.
type hitHeap []hit

func (h hitHeap) Len() int { return len(h) }
func (h hitHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score < h[j].score
	}
	return h[i].idx > h[j].idx // 同分：序号大者更差先淘汰，保稳定
}
func (h hitHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *hitHeap) Push(x any)   { *h = append(*h, x.(hit)) }
func (h *hitHeap) Pop() (v any) { old := *h; v = old[len(old)-1]; *h = old[:len(old)-1]; return v }
func (h *hitHeap) pushOrEvict(x hit, k int) {
	if h.Len() < k {
		heap.Push(h, x)
		return
	}
	if (*h)[0].worseThan(x) {
		heap.Pop(h)
		heap.Push(h, x)
	}
}

// worseThan 稳定序下的比较：分数更低更差，同分序号更大更差.
// [EN] Stable ordering: lower score worse; on ties, larger index worse.
func (a hit) worseThan(b hit) bool {
	if a.score != b.score {
		return a.score < b.score
	}
	return a.idx > b.idx
}

// scanEntries 单片扫描：过滤 + dot 计分 + 维护片内 Top-K 堆.
// 维度相等走预归一化快路径；不等回退原逐条余弦（保留旧截断语义）.
// [EN] Scan one shard: filter + dot scoring + shard-local top-K heap.
// Equal dims take the fast path; otherwise fall back per-entry.
func scanEntries(entries []entry, base int, q []float64, topK int, filters []llmx.Filter) []hit {
	var h hitHeap
	for i := range entries {
		e := &entries[i]
		if len(filters) > 0 && !matchFilters(e.doc.Metadata, filters) {
			continue
		}
		var score float64
		if len(e.unit) == len(q) {
			score = dot(q, e.unit)
		} else {
			score = CosineSimilarity(q, e.vector)
		}
		h.pushOrEvict(hit{score: score, idx: base + i, doc: e.doc}, topK)
	}
	return h
}

// searchSerial 小库单线程路径（避免 goroutine 开销反噬）.
// [EN] Serial path for small stores.
func searchSerial(entries []entry, q []float64, topK int, filters []llmx.Filter) []llmx.Document {
	return hitsToDocs(scanEntries(entries, 0, q, topK, filters))
}

// searchSharded 大库分片并行路径：每 worker 一片局部堆，完成后合并全局堆.
// [EN] Sharded parallel path: per-worker local heaps, then merge.
func searchSharded(entries []entry, q []float64, topK int, filters []llmx.Filter) []llmx.Document {
	workers := runtime.GOMAXPROCS(0)
	if workers > ShardMaxWorkers {
		workers = ShardMaxWorkers
	}
	if n := len(entries); workers > 1 && n/workers >= ShardThreshold/2 {
		chunk := (n + workers - 1) / workers
		local := make([][]hit, workers)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			lo := w * chunk
			if lo >= n {
				local = local[:w]
				break
			}
			hi := lo + chunk
			if hi > n {
				hi = n
			}
			wg.Add(1)
			go func(w, lo, hi int) {
				defer wg.Done()
				local[w] = scanEntries(entries[lo:hi], lo, q, topK, filters)
			}(w, lo, hi)
		}
		wg.Wait()

		var merged hitHeap
		for _, h := range local {
			for _, x := range h {
				merged.pushOrEvict(x, topK)
			}
		}
		return hitsToDocs(merged)
	}
	return searchSerial(entries, q, topK, filters)
}

// hitsToDocs 堆结果降序输出（分数降序，同分写入序升序——与原
// sort.SliceStable 语义一致）.
// [EN] Drain the heap in descending order (stable, matching SliceStable).
func hitsToDocs(h []hit) []llmx.Document {
	if len(h) == 0 {
		return nil
	}
	sort.Slice(h, func(i, j int) bool {
		if h[i].score != h[j].score {
			return h[i].score > h[j].score
		}
		return h[i].idx < h[j].idx
	})
	docs := make([]llmx.Document, len(h))
	for i, x := range h {
		docs[i] = x.doc
	}
	return docs
}

// dot 点积（归一化副本之间即余弦相似度）.
// [EN] Dot product (cosine between normalized copies).
func dot(a, b []float64) float64 {
	var sum float64
	for i := range a {
		if i >= len(b) {
			break
		}
		sum += a[i] * b[i]
	}
	return sum
}

// normalizeCopy 归一化副本（零向量原样拷贝，dot 恒 0 与原兜底一致）.
// [EN] Normalized copy (zero vectors pass through; dot stays 0).
func normalizeCopy(v []float64) []float64 {
	out := make([]float64, len(v))
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i, x := range v {
			out[i] = x / norm
		}
	} else {
		copy(out, v)
	}
	return out
}

// matchFilters 元数据等值过滤（单 Filter 内多 KV 为 AND，多 Filter 之间亦为 AND）.
// [EN] Metadata equality filter (AND within and across filters).
func matchFilters(metadata map[string]any, filters []llmx.Filter) bool {
	for _, f := range filters {
		for k, want := range f {
			got, ok := metadata[k]
			if !ok || got != want {
				return false
			}
		}
	}
	return true
}

// 编译期断言：实现 llmx.VectorStore 契约.
// [EN] Compile-time assertion of the llmx.VectorStore contract.
var _ llmx.VectorStore = (*Store)(nil)
