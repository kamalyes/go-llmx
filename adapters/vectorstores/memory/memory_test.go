/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 21:58:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 21:58:00
 * @FilePath: \go-llmx\adapters\vectorstores\memory\memory_test.go
 * @Description: 内存向量库存储与检索测试 —— 写入校验/Top-K 排序/
 * 元数据过滤/并发安全. 相似度数学见 math_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcmemory

import (
	"context"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddDocuments_CountMismatch(t *testing.T) {
	s := New()
	err := s.AddDocuments(context.Background(),
		[]llmx.Document{{PageContent: "a"}},
		[][]float64{{1, 0}, {0, 1}})
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)
	assert.Equal(t, 0, s.Len())
}

func TestAddDocuments_EmptyVector(t *testing.T) {
	s := New()
	err := s.AddDocuments(context.Background(),
		[]llmx.Document{{PageContent: "a"}},
		[][]float64{nil})
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)
	assert.Equal(t, 0, s.Len())
}

func TestSimilaritySearch_TopKOrdering(t *testing.T) {
	s := New()
	docs := []llmx.Document{
		{PageContent: "猫"},
		{PageContent: "狗"},
		{PageContent: "汽车"},
	}
	vectors := [][]float64{
		{1, 0},  // 与查询同向（相似度 1）
		{0.7, 0.7},
		{0, 1},  // 与查询正交（相似度 0）
	}
	require.NoError(t, s.AddDocuments(context.Background(), docs, vectors))

	// 查询 [1,0] → 按 [猫, 狗, 汽车] 降序
	got, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "猫", got[0].PageContent)
	assert.Equal(t, "狗", got[1].PageContent)
	assert.Equal(t, "汽车", got[2].PageContent)

	// topK 截断
	got2, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 2)
	require.NoError(t, err)
	require.Len(t, got2, 2)
	assert.Equal(t, "猫", got2[0].PageContent)
}

func TestSimilaritySearch_DefaultTopK(t *testing.T) {
	// topK <= 0 → DefaultTopK 兜底
	s := New()
	docs := make([]llmx.Document, DefaultTopK+3)
	vectors := make([][]float64, DefaultTopK+3)
	for i := range docs {
		docs[i] = llmx.Document{PageContent: "doc"}
		vectors[i] = []float64{1, float64(i)}
	}
	require.NoError(t, s.AddDocuments(context.Background(), docs, vectors))

	got, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 0)
	require.NoError(t, err)
	assert.Len(t, got, DefaultTopK)
}

func TestSimilaritySearch_Filters(t *testing.T) {
	s := New()
	docs := []llmx.Document{
		{PageContent: "技术文档", Metadata: map[string]any{"source": "wiki", "lang": "zh"}},
		{PageContent: "新闻", Metadata: map[string]any{"source": "news", "lang": "zh"}},
		{PageContent: "英文文档", Metadata: map[string]any{"source": "wiki", "lang": "en"}},
	}
	vectors := [][]float64{{1, 0}, {0.9, 0.1}, {0.8, 0.2}}
	require.NoError(t, s.AddDocuments(context.Background(), docs, vectors))

	// 单过滤：source=wiki → 技术文档 + 英文文档
	got, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 3, llmx.Filter{"source": "wiki"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "技术文档", got[0].PageContent)

	// 多 KV AND：source=wiki + lang=zh → 仅技术文档
	got2, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 3,
		llmx.Filter{"source": "wiki", "lang": "zh"})
	require.NoError(t, err)
	require.Len(t, got2, 1)
	assert.Equal(t, "技术文档", got2[0].PageContent)

	// 多 Filter AND
	got3, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 3,
		llmx.Filter{"source": "wiki"}, llmx.Filter{"lang": "en"})
	require.NoError(t, err)
	require.Len(t, got3, 1)
	assert.Equal(t, "英文文档", got3[0].PageContent)

	// 无命中过滤
	got4, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 3, llmx.Filter{"source": "blog"})
	require.NoError(t, err)
	assert.Empty(t, got4)
}

func TestSimilaritySearch_InvalidQuery(t *testing.T) {
	s := New()
	_, err := s.SimilaritySearch(context.Background(), nil, 3)
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)
}

func TestSimilaritySearch_EmptyStore(t *testing.T) {
	s := New()
	got, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 3)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestConcurrentAccess(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = s.AddDocuments(context.Background(),
				[]llmx.Document{{PageContent: "并发"}},
				[][]float64{{1, float64(n)}})
			_, _ = s.SimilaritySearch(context.Background(), []float64{1, 0}, 2)
			_ = s.Len()
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 8, s.Len())
}
