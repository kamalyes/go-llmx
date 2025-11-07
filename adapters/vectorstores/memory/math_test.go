/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 22:03:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 22:03:00
 * @FilePath: \go-llmx\adapters\vectorstores\memory\math_test.go
 * @Description: 向量相似度数学测试 —— 同向/正交/反向/零向量/维度不齐兜底
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcmemory

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCosineSimilarity_IdenticalDirection(t *testing.T) {
	// 同向不同模 → 1
	s := CosineSimilarity([]float64{1, 0}, []float64{2, 0})
	assert.InDelta(t, 1.0, s, 1e-9)
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	// 正交 → 0
	s := CosineSimilarity([]float64{1, 0}, []float64{0, 1})
	assert.InDelta(t, 0.0, s, 1e-9)
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	// 反向 → -1
	s := CosineSimilarity([]float64{1, 0}, []float64{-1, 0})
	assert.InDelta(t, -1.0, s, 1e-9)
}

func TestCosineSimilarity_Scaled(t *testing.T) {
	// 已知角度：45° → √2/2
	s := CosineSimilarity([]float64{1, 0}, []float64{1, 1})
	assert.InDelta(t, math.Sqrt2/2, s, 1e-9)
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	// 零向量 → 0 兜底（避免 NaN）
	assert.Equal(t, 0.0, CosineSimilarity([]float64{0, 0}, []float64{1, 0}))
	assert.Equal(t, 0.0, CosineSimilarity([]float64{1, 0}, []float64{0, 0}))
}

func TestCosineSimilarity_DimensionMismatch(t *testing.T) {
	// 维度不齐 → 按前缀对齐计算
	s := CosineSimilarity([]float64{1, 0}, []float64{1, 0, 1})
	assert.InDelta(t, 1.0, s, 1e-9)

	// 反向不齐（a 长于 b）→ 同样前缀对齐
	s2 := CosineSimilarity([]float64{1, 0, 1}, []float64{0, 1})
	assert.InDelta(t, 0.0, s2, 1e-9)

	// 完全无交集维度（空切片）→ 0
	assert.Equal(t, 0.0, CosineSimilarity(nil, []float64{1, 0}))
	assert.Equal(t, 0.0, CosineSimilarity([]float64{1, 0}, nil))
}
