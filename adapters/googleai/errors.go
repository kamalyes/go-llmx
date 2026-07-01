/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-01 20:13:29
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-01 20:18:53
 * @FilePath: \go-llmx\adapters\googleai\errors.go
 * @Description: Google AI 适配器错误差异 —— wire 错误体解析 + Classifier 实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgoogleai

import (
	"encoding/json"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// wireAPIError Gemini 协议的错误响应体结构.
// [EN] Error response structure of the Gemini protocol.
type wireAPIError struct {
	// Error 错误详情容器.
	// [EN] Error detail container.
	Error *wireErrorBody `json:"error,omitempty"`
}

// wireErrorBody 错误详情（Google API 标准形态）.
// [EN] Error detail (Google API standard shape).
type wireErrorBody struct {
	// Code HTTP 状态码数字形态（400/401/429...）.
	// [EN] HTTP status code in numeric form.
	Code int `json:"code"`

	// Message 人类可读错误信息.
	// [EN] Human-readable error message.
	Message string `json:"message"`

	// Status Google 错误状态（INVALID_ARGUMENT / UNAUTHENTICATED / RESOURCE_EXHAUSTED...）.
	// [EN] Google error status.
	Status string `json:"status"`
}

// googleaiClassifier Gemini 协议的错误差异注入.
// [EN] Gemini-specific error classification.
type googleaiClassifier struct{}

// ParseErrorBody 实现 adapter.Classifier（非 JSON 回退原始文本）.
// [EN] Implement adapter.Classifier (raw text fallback).
func (googleaiClassifier) ParseErrorBody(body string) *adapter.ErrorBody {
	var we wireAPIError
	if err := json.Unmarshal([]byte(body), &we); err != nil || we.Error == nil {
		return &adapter.ErrorBody{Message: body, Type: adapter.ErrorTypeHTTP}
	}
	return &adapter.ErrorBody{Type: we.Error.Status, Message: we.Error.Message}
}

// MapErrorType 实现 adapter.Classifier（Google 状态映射 llmx 语义错误）.
// [EN] Implement adapter.Classifier (Google status to llmx semantic errors).
func (googleaiClassifier) MapErrorType(status string) error {
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
func (googleaiClassifier) MapSpecial(error) error { return nil }
