/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 20:37:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 20:37:00
 * @FilePath: \go-llmx\adapters\ollama\errors.go
 * @Description: Ollama 适配器错误差异 —— wire 错误体解析 + Classifier 实现 +
 * 流中 error 帧认领. 公共映射骨架（网络/状态分类/流终止）见核心库 adapter 包
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollama

import (
	"encoding/json"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// wireErrorBody 错误响应体（Ollama 错误体为字符串字段，无类型体系）.
// [EN] Error response body (message only, no type system).
type wireErrorBody struct {
	// Error 人类可读错误信息.
	// [EN] Human-readable error message.
	Error string `json:"error"`
}

// streamError 流中 error 帧错误（NDJSON 任意帧可携带 error 字段）.
// [EN] Mid-stream error (any NDJSON frame may carry an error field).
type streamError struct {
	// message 协议错误信息.
	// [EN] Protocol error message.
	message string
}

// Error 实现 error.
// [EN] Implement error.
func (e *streamError) Error() string {
	return errorTypeOllama + ": " + e.message
}

// ollamaClassifier Ollama 协议的错误差异注入.
// [EN] Ollama error classification.
type ollamaClassifier struct{}

// ParseErrorBody 实现 adapter.Classifier（非 JSON 回退原始文本）.
// [EN] Implement adapter.Classifier (raw text fallback).
func (ollamaClassifier) ParseErrorBody(body string) *adapter.ErrorBody {
	var we wireErrorBody
	if err := json.Unmarshal([]byte(body), &we); err != nil || we.Error == "" {
		return &adapter.ErrorBody{Message: body, Type: adapter.ErrorTypeHTTP}
	}
	return &adapter.ErrorBody{Type: errorTypeOllama, Message: we.Error}
}

// MapErrorType 实现 adapter.Classifier（协议无类型体系，恒走状态分类兜底）.
// [EN] Implement adapter.Classifier (no type mapping; always falls back to status class).
func (ollamaClassifier) MapErrorType(string) error { return nil }

// MapSpecial 实现 adapter.Classifier（认领流中 error 帧）.
// [EN] Implement adapter.Classifier (claim mid-stream error frames).
func (ollamaClassifier) MapSpecial(err error) error {
	if se, ok := err.(*streamError); ok {
		return fmt.Errorf("%w: %s", llmx.ErrAPIServerError, se.message)
	}
	return nil
}
