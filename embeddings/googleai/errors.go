/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-09 21:07:26
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:07:26
 * @FilePath: \go-llmx\embeddings\googleai\errors.go
 * @Description: Google AI 嵌入适配器错误差异 —— Google 标准错误体解析 + Classifier 实现.
 * 状态字面量映射与对话适配器同构（RESOURCE_EXHAUSTED → 限流等）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgembed

import (
	"encoding/json"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// wireAPIError 协议的错误响应体容器（Google 标准形态）.
// [EN] Error response container (standard Google shape).
type wireAPIError struct {
	// Error 错误详情.
	// [EN] Error detail.
	Error *wireErrorBody `json:"error,omitempty"`
}

// wireErrorBody 错误详情（code/status/message 三元组）.
// [EN] Error detail (code/status/message triple).
type wireErrorBody struct {
	// Code HTTP 状态码数值形态.
	// [EN] Numeric HTTP status.
	Code int `json:"code"`

	// Status 状态字面量（UNAUTHENTICATED / RESOURCE_EXHAUSTED / ...）.
	// [EN] Status literal.
	Status string `json:"status"`

	// Message 人类可读错误信息.
	// [EN] Human-readable message.
	Message string `json:"message"`
}

// classifier 嵌入协议的错误差异注入.
// [EN] Embedding error classification.
type classifier struct{}

// ParseErrorBody 实现 adapter.Classifier（非 JSON 回退原始文本）.
// [EN] Implement adapter.Classifier (raw text fallback).
func (classifier) ParseErrorBody(body string) *adapter.ErrorBody {
	var we wireAPIError
	if err := json.Unmarshal([]byte(body), &we); err != nil || we.Error == nil {
		return &adapter.ErrorBody{Message: body, Type: adapter.ErrorTypeHTTP}
	}
	return &adapter.ErrorBody{Type: we.Error.Status, Message: we.Error.Message}
}

// MapErrorType 实现 adapter.Classifier（Google 状态字面量 → llmx 哨兵）.
// [EN] Implement adapter.Classifier (Google status literal to llmx sentinel).
func (classifier) MapErrorType(status string) error {
	switch status {
	case "UNAUTHENTICATED", "PERMISSION_DENIED":
		return llmx.ErrUnauthorized
	case "RESOURCE_EXHAUSTED":
		return llmx.ErrRateLimited
	case "NOT_FOUND":
		return llmx.ErrUnsupportedOperation
	default:
		return nil
	}
}

// MapSpecial 实现 adapter.Classifier（无特有错误认领）.
// [EN] Implement adapter.Classifier (nothing special to claim).
func (classifier) MapSpecial(error) error { return nil }
