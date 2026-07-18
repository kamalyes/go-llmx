/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-11 20:51:32
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-18 10:36:08
 * @FilePath: \go-llmx\embeddings\cohere\constants.go
 * @Description: Cohere 嵌入适配器常量 —— 默认端点/模型/协议路径/input_type 字面量/批量上限
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccembed

const (
	// DefaultBaseURL Cohere 官方端点.
	// [EN] Cohere official endpoint.
	DefaultBaseURL = "https://api.cohere.com/v2"

	// DefaultModel 默认嵌入模型（新一代多语种）.
	// [EN] Default embedding model.
	DefaultModel = "embed-v4.0"

	// EmbedPath 嵌入协议路径（端点变更时仅改此处）.
	// [EN] Embedding protocol path (single source of truth).
	EmbedPath = "/embed"
)

// embedBatchMaxTexts 单请求文本数上限（Cohere 官方 /v2/embed 限制 96 条，
// 超出自动拆批有界并行）.
// [EN] Max texts per request (official Cohere limit; excess batches out
// bounded-parallel).
const embedBatchMaxTexts = 96

// input_type 字面量（非对称检索语义：索引文档与查询分开优化）.
// [EN] input_type literals (asymmetric retrieval semantics).
const (
	inputTypeDocument = "search_document"
	inputTypeQuery    = "search_query"
)

// embeddingTypesFloats 请求的向量形态（仅 float，量化形态不适用通用检索）.
// [EN] Requested vector form (float only; quantized forms don't fit general retrieval).
const embeddingTypesFloats = "float"
