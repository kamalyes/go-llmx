/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 21:29:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 21:36:00
 * @FilePath: \go-llmx\adapters\vectorstores\memory\memory.go
 * @Description: 内存向量库 —— 进程内余弦相似度检索（单测/原型/小规模数据）.
 * 零依赖并发安全实现；相似度数学见 math.go，生产规模见 redis/pgvector 适配器
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcmemory

import (
	"context"
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

	// vector 嵌入向量.
	// [EN] The embedding vector.
	vector []float64
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
	for i, d := range docs {
		s.entries = append(s.entries, entry{doc: d, vector: append([]float64(nil), vectors[i]...)})
	}
	return nil
}

// SimilaritySearch 实现 llmx.VectorStore（余弦相似度 Top-K，filters 可选）.
// [EN] Implement llmx.VectorStore (cosine top-K, filters optional).
func (s *Store) SimilaritySearch(_ context.Context, query []float64, topK int, filters ...llmx.Filter) ([]llmx.Document, error) {
	if len(query) < MinVectorDim {
		return nil, llmx.ErrInvalidVectors
	}
	if topK <= 0 {
		topK = DefaultTopK
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	type scored struct {
		doc   llmx.Document
		score float64
	}
	var hits []scored
	for _, e := range s.entries {
		if !matchFilters(e.doc.Metadata, filters) {
			continue
		}
		hits = append(hits, scored{doc: e.doc, score: CosineSimilarity(query, e.vector)})
	}

	// 相似度降序，稳定排序保持同分写入顺序
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > topK {
		hits = hits[:topK]
	}

	docs := make([]llmx.Document, 0, len(hits))
	for _, h := range hits {
		docs = append(docs, h.doc)
	}
	return docs, nil
}

// Len 返回当前存储条数.
// [EN] Return the number of stored entries.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
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
