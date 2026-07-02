/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 20:13:52
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 20:25:31
 * @FilePath: \go-llmx\adapters\mistral\errors.go
 * @Description: Mistral 适配器错误差异 —— 扁平错误体解析 + Classifier 实现.
 * Mistral 错误体不带 error 包裹（区别于 OpenAI 的 {"error": {...}} 形态）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmistral

import (
	"encoding/json"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// wireAPIError Mistral 协议的错误响应体结构（扁平形态）.
// [EN] Error response structure (flat shape).
type wireAPIError struct {
	// Message 人类可读错误信息.
	// [EN] Human-readable error message.
	Message string `json:"message"`

	// Type 错误类型（invalid_request_error / rate_limit_error 等）.
	// [EN] Error type.
	Type string `json:"type"`

	// Code 平台错误码（字符串或数字，宽松解码）.
	// [EN] Platform error code (string or number, loosely decoded).
	Code any `json:"code"`
}

// mistralClassifier Mistral 协议的错误差异注入.
// [EN] Mistral-specific error classification.
type mistralClassifier struct{}

// ParseErrorBody 实现 adapter.Classifier（非 JSON 回退原始文本）.
// [EN] Implement adapter.Classifier (raw text fallback).
func (mistralClassifier) ParseErrorBody(body string) *adapter.ErrorBody {
	var we wireAPIError
	if err := json.Unmarshal([]byte(body), &we); err != nil || we.Message == "" {
		return &adapter.ErrorBody{Message: body, Type: adapter.ErrorTypeHTTP}
	}
	return &adapter.ErrorBody{Type: we.Type, Message: we.Message}
}

// MapErrorType 实现 adapter.Classifier（Mistral 错误类型映射）.
// [EN] Implement adapter.Classifier.
func (mistralClassifier) MapErrorType(errType string) error {
	switch errType {
	case errorTypeInvalidRequest:
		return llmx.ErrInvalidRequest
	case errorTypeRateLimit:
		return llmx.ErrRateLimited
	default:
		return nil
	}
}

// MapSpecial 实现 adapter.Classifier（无特有错误认领）.
// [EN] Implement adapter.Classifier (nothing special to claim).
func (mistralClassifier) MapSpecial(error) error { return nil }
