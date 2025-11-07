/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 21:38:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 21:38:00
 * @FilePath: \go-llmx\adapters\vectorstores\memory\math.go
 * @Description: 向量相似度数学 —— 余弦相似度（长度不等按前缀对齐截断，
 * 维度不匹配视为不可比兜底 0）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcmemory

import "math"

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
