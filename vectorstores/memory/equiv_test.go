/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-17 09:12:05
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-17 09:12:05
 * @FilePath: \go-llmx\vectorstores\memory\equiv_test.go
 * @Description: 检索路径等价性测试 —— 优化实现（预归一化+堆+分片）
 * 与优化前形态（全量 CosineSimilarity + SliceStable）在含同分/
 * 零向量/维度不等/filters/大库分片的数据上输出必须逐位一致
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmemory

import (
	"context"
	"math/rand"
	"sort"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacySearch 优化前形态（对照组）：全量逐条 CosineSimilarity +
// sort.SliceStable——与 langchaingo 内存库同款结构.
// [EN] Pre-optimization shape (control group).
func legacySearch(s *Store, query []float64, topK int, filters ...llmx.Filter) []llmx.Document {
	if topK <= 0 {
		topK = DefaultTopK
	}
	s.mu.RLock()
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
	s.mu.RUnlock()

	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > topK {
		hits = hits[:topK]
	}
	docs := make([]llmx.Document, 0, len(hits))
	for _, h := range hits {
		docs = append(docs, h.doc)
	}
	return docs
}

// assertDocSeq 比较两结果序列（按内容定位）.
// [EN] Compare result sequences.
func assertDocSeq(t *testing.T, want, got []llmx.Document) {
	t.Helper()
	if !assert.Len(t, got, len(want)) {
		return
	}
	for i := range want {
		assert.Equal(t, want[i].PageContent, got[i].PageContent, "第 %d 位不同", i)
	}
}

// TestEquiv_TiesAndEdgeCases 同分/零向量/维度不等/filters 全覆盖.
// [EN] Ties, zero vectors, unequal dims, filters.
func TestEquiv_TiesAndEdgeCases(t *testing.T) {
	s := New()
	docs := []llmx.Document{
		{PageContent: "zero-1", Metadata: llmx.Filter{"src": "a"}},
		{PageContent: "dup-a"},
		{PageContent: "dup-b"}, // 与 dup-a 同向同分
		{PageContent: "short", Metadata: llmx.Filter{"src": "a"}}, // 维度不等（慢路径）
		{PageContent: "hit", Metadata: llmx.Filter{"src": "a"}},
		{PageContent: "ortho"},
	}
	vectors := [][]float64{
		{0, 0},     // 零向量 → 0
		{1, 1},     // 与 dup-b 同向
		{2, 2},     // 与 dup-a 同分
		{1},        // 维度 1 截断语义
		{0.9, 0.1}, // 最接近查询
		{0, 1},     // 正交
	}
	require.NoError(t, s.AddDocuments(context.Background(), docs, vectors))

	for _, topK := range []int{1, 3, 6, 99} {
		want := legacySearch(s, []float64{1, 0.1}, topK)
		got, err := s.SimilaritySearch(context.Background(), []float64{1, 0.1}, topK)
		require.NoError(t, err)
		assertDocSeq(t, want, got)

		// filters 路径
		wantF := legacySearch(s, []float64{1, 0.1}, topK, llmx.Filter{"src": "a"})
		gotF, err := s.SimilaritySearch(context.Background(), []float64{1, 0.1}, topK, llmx.Filter{"src": "a"})
		require.NoError(t, err)
		assertDocSeq(t, wantF, gotF)
	}
}

// TestEquiv_LargeSharded 超过分片阈值的大库等价（含同分冲突）.
// [EN] Equivalence above the shard threshold.
func TestEquiv_LargeSharded(t *testing.T) {
	s := New()
	rng := rand.New(rand.NewSource(42))

	n := ShardThreshold + 1234
	docs := make([]llmx.Document, n)
	vectors := make([][]float64, n)
	for i := 0; i < n; i++ {
		v := []float64{rng.Float64() - 0.5, rng.Float64() - 0.5, rng.Float64() - 0.5}
		vectors[i] = v
		docs[i] = llmx.Document{PageContent: "doc", Metadata: llmx.Filter{"seq": i}}
	}
	// 批量制造同分：三分之一直接重复首条向量
	for i := 1; i < n; i *= 3 {
		vectors[i] = vectors[0]
	}
	require.NoError(t, s.AddDocuments(context.Background(), docs, vectors))

	query := []float64{0.3, -0.2, 0.9}
	for _, topK := range []int{1, 10, 200} {
		want := legacySearch(s, query, topK)
		got, err := s.SimilaritySearch(context.Background(), query, topK)
		require.NoError(t, err)
		assertDocSeq(t, want, got)
	}
}
