/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 22:13:52
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 22:47:11
 * @FilePath: \go-llmx\adapters\cohere\errors.go
 * @Description: Cohere 适配器错误差异 —— v4 错误体解析 + Classifier 实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccohere

import (
	"encoding/json"

	"github.com/kamalyes/go-llmx/adapter"
)

// wireAPIError Cohere 协议的错误响应体结构.
// [EN] Error response structure of the Cohere protocol.
type wireAPIError struct {
	// Message 人类可读错误信息.
	// [EN] Human-readable error message.
	Message string `json:"message"`
}

// cohereClassifier Cohere 协议的错误差异注入（错误体仅 message，类型走状态分类兜底）.
// [EN] Cohere-specific error classification (message-only body; status fallback).
type cohereClassifier struct{}

// ParseErrorBody 实现 adapter.Classifier（非 JSON 回退原始文本）.
// [EN] Implement adapter.Classifier (raw text fallback).
func (cohereClassifier) ParseErrorBody(body string) *adapter.ErrorBody {
	var we wireAPIError
	if err := json.Unmarshal([]byte(body), &we); err != nil || we.Message == "" {
		return &adapter.ErrorBody{Message: body, Type: adapter.ErrorTypeHTTP}
	}
	return &adapter.ErrorBody{Message: we.Message, Type: "cohere_error"}
}

// MapErrorType 实现 adapter.Classifier（无类型字段，恒走状态分类兜底）.
// [EN] Implement adapter.Classifier (no type field; status fallback).
func (cohereClassifier) MapErrorType(string) error { return nil }

// MapSpecial 实现 adapter.Classifier（无特有错误认领）.
// [EN] Implement adapter.Classifier (nothing special to claim).
func (cohereClassifier) MapSpecial(error) error { return nil }
