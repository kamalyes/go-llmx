/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-09 21:03:47
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:03:47
 * @FilePath: \go-llmx\embeddings\googleai\constants.go
 * @Description: Google AI 嵌入适配器常量 —— 默认端点/模型/协议方法/批次上限
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgembed

const (
	// DefaultBaseURL Google AI 端点（与对话适配器同源）.
	// [EN] Google AI endpoint (same origin as the chat adapter).
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

	// DefaultModel 默认嵌入模型（Gemini 新一代嵌入）.
	// [EN] Default embedding model.
	DefaultModel = "gemini-embedding-001"

	// maxBatchRequests batchEmbedContents 单批请求上限（Google 官方限制）.
	// [EN] Per-batch request limit of batchEmbedContents (official Google limit).
	maxBatchRequests = 100
)

// 协议方法字面量（模型名拼入 URL：/models/{model}:method）.
// [EN] Protocol method literals (model name embedded in the URL).
const (
	methodEmbedContent       = ":embedContent"
	methodBatchEmbedContents = ":batchEmbedContents"
)

// task type 字面量（非对称检索语义：索引文档与查询分开优化）.
// [EN] Task type literals (asymmetric retrieval semantics).
const (
	taskTypeDocument = "RETRIEVAL_DOCUMENT"
	taskTypeQuery    = "RETRIEVAL_QUERY"
)
