/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-01 20:37:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-01 20:37:00
 * @FilePath: \go-llmx\adapters\openai\errors.go
 * @Description: OpenAI 兼容适配器错误差异 —— wire 错误体解析 + Classifier 实现.
 * 公共映射骨架（网络/状态分类/流终止）见核心库 adapter 包
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"encoding/json"

	"github.com/kamalyes/go-llmx/adapter"
)

// wireAPIError OpenAI 兼容协议的错误响应体结构.
// [EN] Error response structure of the OpenAI-compatible protocol.
type wireAPIError struct {
	// Error 错误详情容器.
	// [EN] Error detail container.
	Error *wireErrorBody `json:"error,omitempty"`
}

// wireErrorBody 错误详情.
// [EN] Error detail body.
type wireErrorBody struct {
	// Message 人类可读错误信息.
	// [EN] Human-readable error message.
	Message string `json:"message"`

	// Type 错误类型（invalid_request_error / rate_limit_error 等）.
	// [EN] Error type.
	Type string `json:"type"`

	// Code 平台错误码（可能为字符串或数字，宽松解码）.
	// [EN] Platform error code (string or number, loosely decoded).
	Code any `json:"code"`
}

// openaiClassifier OpenAI 兼容协议的错误差异注入.
// [EN] OpenAI-compatible error classification.
type openaiClassifier struct{}

// ParseErrorBody 实现 adapter.Classifier（非 JSON 回退原始文本）.
// [EN] Implement adapter.Classifier (raw text fallback).
func (openaiClassifier) ParseErrorBody(body string) *adapter.ErrorBody {
	var we wireAPIError
	if err := json.Unmarshal([]byte(body), &we); err != nil || we.Error == nil {
		return &adapter.ErrorBody{Message: body, Type: adapter.ErrorTypeHTTP}
	}
	return toErrorBody(we.Error)
}

// MapErrorType 实现 adapter.Classifier（无协议特有类型映射，恒走状态分类兜底）.
// [EN] Implement adapter.Classifier (no type mapping; always falls back to status class).
func (openaiClassifier) MapErrorType(string) error { return nil }

// MapSpecial 实现 adapter.Classifier（无特有错误认领）.
// [EN] Implement adapter.Classifier (nothing special to claim).
func (openaiClassifier) MapSpecial(error) error { return nil }

// toErrorBody wire 错误详情 → 归一错误载荷.
// [EN] Convert a wire error to the normalized payload.
func toErrorBody(we *wireErrorBody) *adapter.ErrorBody {
	return &adapter.ErrorBody{Type: we.Type, Message: we.Message}
}
