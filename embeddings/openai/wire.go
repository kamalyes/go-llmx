/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 22:19:00
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:38:16
 * @FilePath: \go-llmx\embeddings\openai\wire.go
 * @Description: OpenAI 兼容嵌入协议编解码 —— wire 请求/响应结构
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcembed

import (
	"github.com/kamalyes/go-llmx/adapter"
)

// wireRequest /embeddings 请求体.
// [EN] /embeddings request body.
type wireRequest struct {
	// Model 嵌入模型名.
	// [EN] Embedding model name.
	Model string `json:"model"`

	// Input 输入文本（单条或批量，恒为数组形态）.
	// [EN] Input texts (single or batch, always an array).
	Input []string `json:"input"`
}

// wireResponse /embeddings 响应体.
// [EN] /embeddings response body.
type wireResponse struct {
	// Data 向量载荷集合（index 与输入顺序对应）.
	// [EN] Vector payloads (index matches input order).
	Data []wirePayload `json:"data"`

	// Error 200 状态下网关仍可能注入的错误字段.
	// [EN] Error field some gateways inject even on 200.
	Error *wireErrorBody `json:"error,omitempty"`
}

// wirePayload 单条向量载荷.
// [EN] A single embedding payload.
type wirePayload struct {
	// Index 输入序号（与请求 Input 下标对应）.
	// [EN] Input index (matches the request Input slice).
	Index int `json:"index"`

	// Embedding 向量.
	// [EN] The vector.
	Embedding []float64 `json:"embedding"`
}

// wireErrorBody 错误详情（与对话协议同形态）.
// [EN] Error body (same shape as the chat protocol).
type wireErrorBody struct {
	// Message 人类可读错误信息.
	// [EN] Human-readable message.
	Message string `json:"message"`

	// Type 错误类型.
	// [EN] Error type.
	Type string `json:"type"`
}

// toErrorBody wire 错误详情 → 归一错误载荷.
// [EN] Convert a wire error to the normalized payload.
func toErrorBody(we *wireErrorBody) *adapter.ErrorBody {
	return &adapter.ErrorBody{Type: we.Type, Message: we.Message}
}
