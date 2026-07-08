/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 21:38:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-08 20:57:33
 * @FilePath: \go-llmx\vectorstores\memory\math.go
 * @Description: 向量相似度数学 —— 余弦相似度（长度不等按前缀对齐截断，
 * 维度不匹配视为不可比兜底 0）+ 向量合并（加权平均归一化）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcmemory

import (
	"math"

	llmx "github.com/kamalyes/go-llmx"
)

// CosineSimilarity 余弦相似度（[-1, 1]，1 为方向完全一致）.
// [EN] Cosine similarity ([-1, 1]; 1 means identical direction).
//
// 维度不等或零向量 → 0（不可比兜底，避免 NaN 污染排序）
func CosineSimilarity(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}

	var dot, na, nb float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// CombineVectors 合并多维向量（加权和平均后归一化为单位向量）.
// [EN] Combine vectors (weighted average, then normalize to unit length).
//
// 典型用途：父文档检索将同一父块的多个子块向量合并为代表向量；
// weights 为 nil 时等权（全 1）。维度不齐或权重数不匹配返回
// ErrInvalidVectors，零和权重（全空输入或权重全零）返回 ErrInvalidVectors
func CombineVectors(vectors [][]float64, weights []int) ([]float64, error) {
	if len(vectors) == 0 {
		return nil, llmx.ErrInvalidVectors
	}
	if weights != nil && len(weights) != len(vectors) {
		return nil, llmx.ErrInvalidVectors
	}

	dim := len(vectors[0])
	for _, v := range vectors[1:] {
		if len(v) != dim {
			return nil, llmx.ErrInvalidVectors
		}
	}

	weightSum := 0
	if weights == nil {
		for range vectors {
			weightSum++
		}
	} else {
		for _, w := range weights {
			weightSum += w
		}
	}
	if weightSum == 0 {
		return nil, llmx.ErrInvalidVectors
	}

	combined := make([]float64, dim)
	for i, v := range vectors {
		w := 1.0
		if weights != nil {
			w = float64(weights[i])
		}
		for j := 0; j < dim; j++ {
			combined[j] += v[j] * w
		}
	}
	for j := range combined {
		combined[j] /= float64(weightSum)
	}

	// 归一化为单位向量（零向量原样返回，调用方以余弦兜底处理）
	var norm float64
	for _, x := range combined {
		norm += x * x
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for j := range combined {
			combined[j] /= norm
		}
	}
	return combined, nil
}
