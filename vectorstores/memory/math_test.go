/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 22:03:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-08 20:57:33
 * @FilePath: \go-llmx\vectorstores\memory\math_test.go
 * @Description: 向量相似度数学测试 —— 同向/正交/反向/零向量/维度不齐兜底
 * + 向量合并（等权/加权/归一化/非法输入）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcmemory

import (
	"math"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestCombineVectors_EqualWeights(t *testing.T) {
	// 等权平均：(3,0) 与 (0,3) → 均值 (1.5,1.5) → 归一化 (√2/2, √2/2)
	out, err := CombineVectors([][]float64{{3, 0}, {0, 3}}, nil)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.InDelta(t, math.Sqrt2/2, out[0], 1e-9)
	assert.InDelta(t, math.Sqrt2/2, out[1], 1e-9)
}

func TestCombineVectors_Weighted(t *testing.T) {
	// 加权 3:1：3*(2,0) + 1*(0,2) → (6,2)/4 → 归一化后方向比例 3:1
	out, err := CombineVectors([][]float64{{2, 0}, {0, 2}}, []int{3, 1})
	require.NoError(t, err)
	// 归一化前均值 (1.5, 0.5)，归一化后仍保持 3:1
	assert.InDelta(t, 3*out[1], out[0], 1e-9)
	// 单位向量校验
	var norm float64
	for _, x := range out {
		norm += x * x
	}
	assert.InDelta(t, 1.0, norm, 1e-9)
}

func TestCombineVectors_SingleVector(t *testing.T) {
	// 单向量 → 归一化自身
	out, err := CombineVectors([][]float64{{0, 5}}, nil)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, out[0], 1e-9)
	assert.InDelta(t, 1.0, out[1], 1e-9)
}

func TestCombineVectors_Invalid(t *testing.T) {
	// 空输入
	_, err := CombineVectors(nil, nil)
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)

	// 维度不齐
	_, err = CombineVectors([][]float64{{1, 0}, {1, 0, 0}}, nil)
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)

	// 权重数不匹配
	_, err = CombineVectors([][]float64{{1, 0}}, []int{1, 2})
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)

	// 权重全零（零和）
	_, err = CombineVectors([][]float64{{1, 0}}, []int{0})
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)
}

func TestCombineVectors_ZeroVectorsInput(t *testing.T) {
	// 全零向量输入 → 均值零向量，归一化跳过原样返回（不 NaN）
	out, err := CombineVectors([][]float64{{0, 0}, {0, 0}}, nil)
	require.NoError(t, err)
	assert.Equal(t, []float64{0, 0}, out)
}
