/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-12 21:07:33
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-12 21:23:07
 * @FilePath: \go-llmx\vectorstores\redisvector\constants.go
 * @Description: Redis 向量库常量 —— 与 langchaingo/redisvector 数据格式逐字段对齐
 * （content/content_vector 字段名、doc:<index>: 键前缀），存量索引可平滑读写
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcredisvector

const (
	// DefaultIndex 默认 RediSearch 索引名（与 langchaingo 默认一致，
	// 存量 langchaingo 索引零改造平滑接入）.
	// [EN] Default RediSearch index name (aligned with langchaingo;
	// existing langchaingo indexes work without changes).
	DefaultIndex = "golang_langchain"

	// DefaultDimensions 默认向量维度（text-embedding-3-small）.
	// [EN] Default vector dimension.
	DefaultDimensions = 1536
)

// HASH 字段字面量（langchaingo 同名）.
// [EN] HASH field literals (same names as langchaingo).
const (
	fieldContent = "content"
	fieldVector  = "content_vector"
	fieldScore   = "distance"
)

// HNSW 索引参数（写入侧建索引用；读取兼容 langchaingo 默认 FLAT 索引）.
// [EN] HNSW index parameters (for creation; reads also work with FLAT indexes).
const (
	hnswType       = "FLOAT32"
	hnswM          = 16
	hnswEFConstant = 200
	hnswEFRuntime  = 10
)
