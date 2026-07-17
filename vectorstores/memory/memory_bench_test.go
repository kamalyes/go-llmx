/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-17 09:21:38
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-17 09:21:38
 * @FilePath: \go-llmx\vectorstores\memory\memory_bench_test.go
 * @Description: 检索路径基准 —— 优化实现（预归一化+堆+分片）vs
 * 优化前形态（全量余弦+全量排序，对照组）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmemory

import (
	"context"
	"math/rand"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
)

// benchStore 预置 n 条 dim 维随机向量库（固定种子可复现）.
// [EN] Pre-populated store (fixed seed).
func benchStore(n, dim int) *Store {
	s := New()
	rng := rand.New(rand.NewSource(7))
	docs := make([]llmx.Document, n)
	vectors := make([][]float64, n)
	for i := range docs {
		v := make([]float64, dim)
		for j := range v {
			v[j] = rng.Float64() - 0.5
		}
		vectors[i] = v
		docs[i] = llmx.Document{PageContent: "bench", Metadata: llmx.Filter{"seq": i}}
	}
	if err := s.AddDocuments(context.Background(), docs, vectors); err != nil {
		panic(err)
	}
	return s
}

// benchQuery 固定查询向量（与库同源但不重复）.
// [EN] Fixed query vector.
func benchQuery(dim int) []float64 {
	rng := rand.New(rand.NewSource(99))
	q := make([]float64, dim)
	for i := range q {
		q[i] = rng.Float64() - 0.5
	}
	return q
}

// BenchmarkSearch_Legacy 对照组：优化前形态（全量逐条余弦含范数
// + sort.SliceStable 全量排序）——langchaingo 内存库同款结构.
// [EN] Control: pre-optimization shape.
func BenchmarkSearch_Legacy(b *testing.B) {
	s := benchStore(50_000, 256)
	q := benchQuery(256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if docs := legacySearch(s, q, 10); len(docs) != 10 {
			b.Fatal("bad result")
		}
	}
}

// BenchmarkSearch_Optimized 优化实现（预归一化 dot + Top-K 堆 +
// 50k 条自动触发并发分片）.
// [EN] The optimized implementation.
func BenchmarkSearch_Optimized(b *testing.B) {
	s := benchStore(50_000, 256)
	q := benchQuery(256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		docs, err := s.SimilaritySearch(context.Background(), q, 10)
		if err != nil || len(docs) != 10 {
			b.Fatal("bad result")
		}
	}
}

// BenchmarkSearch_Optimized_Small 小库（2000 条，低于分片阈值）
// 单线程堆路径——验证小库不反噬.
// [EN] Small store below the shard threshold.
func BenchmarkSearch_Optimized_Small(b *testing.B) {
	s := benchStore(2_000, 256)
	q := benchQuery(256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.SimilaritySearch(context.Background(), q, 10); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearch_Legacy_Small 小库对照组.
// [EN] Small-store control.
func BenchmarkSearch_Legacy_Small(b *testing.B) {
	s := benchStore(2_000, 256)
	q := benchQuery(256)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		legacySearch(s, q, 10)
	}
}

// BenchmarkAddDocuments 写入路径（预归一化的额外成本）.
// [EN] Write path (normalization cost).
func BenchmarkAddDocuments(b *testing.B) {
	docs, vectors := func() ([]llmx.Document, [][]float64) {
		rng := rand.New(rand.NewSource(3))
		docs := make([]llmx.Document, 1_000)
		vectors := make([][]float64, 1_000)
		for i := range docs {
			v := make([]float64, 256)
			for j := range v {
				v[j] = rng.Float64() - 0.5
			}
			vectors[i] = v
			docs[i] = llmx.Document{PageContent: "bench"}
		}
		return docs, vectors
	}()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := New()
		if err := s.AddDocuments(context.Background(), docs, vectors); err != nil {
			b.Fatal(err)
		}
	}
}
