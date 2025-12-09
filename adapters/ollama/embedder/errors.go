/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 22:23:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 22:23:00
 * @FilePath: \go-llmx\adapters\ollama\embedder\errors.go
 * @Description: Ollama 嵌入适配器错误差异 —— wire 错误体解析 + Classifier 实现.
 * 公共映射骨架（网络/状态分类）见核心库 adapter 包
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollamaembed

import (
	"encoding/json"

	"github.com/kamalyes/go-llmx/adapter"
)

// wireErrorBody 错误响应体（Ollama 错误体为字符串字段，无类型体系）.
// [EN] Error response body (message only, no type system).
type wireErrorBody struct {
	// Error 人类可读错误信息.
	// [EN] Human-readable error message.
	Error string `json:"error"`
}

// classifier Ollama 嵌入协议的错误差异注入.
// [EN] Ollama embedding error classification.
type classifier struct{}

// ParseErrorBody 实现 adapter.Classifier（非 JSON 回退原始文本）.
// [EN] Implement adapter.Classifier (raw text fallback).
func (classifier) ParseErrorBody(body string) *adapter.ErrorBody {
	var we wireErrorBody
	if err := json.Unmarshal([]byte(body), &we); err != nil || we.Error == "" {
		return &adapter.ErrorBody{Message: body, Type: adapter.ErrorTypeHTTP}
	}
	return &adapter.ErrorBody{Type: errorTypeOllamaEmbed, Message: we.Error}
}

// MapErrorType 实现 adapter.Classifier（协议无类型体系，恒走状态分类兜底）.
// [EN] Implement adapter.Classifier (no type mapping; always falls back to status class).
func (classifier) MapErrorType(string) error { return nil }

// MapSpecial 实现 adapter.Classifier（无特有错误认领）.
// [EN] Implement adapter.Classifier (nothing special to claim).
func (classifier) MapSpecial(error) error { return nil }
