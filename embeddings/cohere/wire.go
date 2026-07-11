/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-11 20:55:16
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-11 20:55:16
 * @FilePath: \go-llmx\embeddings\cohere\wire.go
 * @Description: Cohere 嵌入协议编解码 —— /v2/embed wire 结构（v2 形态）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccembed

// wireRequest /embed 请求体.
// [EN] /embed request body.
type wireRequest struct {
	// Model 嵌入模型名.
	// [EN] Embedding model name.
	Model string `json:"model"`

	// Texts 输入文本（单条或批量，恒为数组形态）.
	// [EN] Input texts (single or batch, always an array).
	Texts []string `json:"texts"`

	// InputType 任务类型（search_document / search_query）.
	// [EN] Task type.
	InputType string `json:"input_type"`

	// EmbeddingTypes 请求的向量形态（仅 float）.
	// [EN] Requested vector form (float only).
	EmbeddingTypes []string `json:"embedding_types"`
}

// wireResponse /embed 响应体.
// [EN] /embed response body.
type wireResponse struct {
	// Embeddings 向量载荷（float 形态数组，顺序与输入一致）.
	// [EN] Vector payloads (float form, order matches the input).
	Embeddings wireEmbeddingSet `json:"embeddings"`
}

// wireEmbeddingSet 多形态向量集合（本适配器仅消费 float）.
// [EN] Multi-form vector set (only float is consumed).
type wireEmbeddingSet struct {
	// Float float64 向量集合.
	// [EN] float64 vectors.
	Float [][]float64 `json:"float"`
}

// wireErrorBody 错误详情（与对话协议同形态）.
// [EN] Error body (same shape as the chat protocol).
type wireErrorBody struct {
	// Message 人类可读错误信息.
	// [EN] Human-readable message.
	Message string `json:"message"`
}
