/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 22:26:00
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:38:16
 * @FilePath: \go-llmx\embeddings\openai\errors.go
 * @Description: OpenAI 兼容嵌入适配器错误差异 —— wire 错误体解析 + Classifier 实现.
 * 公共映射骨架（网络/状态分类）见核心库 adapter 包
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcembed

import (
	"encoding/json"

	"github.com/kamalyes/go-llmx/adapter"
)

// wireAPIError 协议的错误响应体容器.
// [EN] Error response container of the protocol.
type wireAPIError struct {
	// Error 错误详情.
	// [EN] Error detail.
	Error *wireErrorBody `json:"error,omitempty"`
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
	return toErrorBody(we.Error)
}

// MapErrorType 实现 adapter.Classifier（无协议特有类型映射，恒走状态分类兜底）.
// [EN] Implement adapter.Classifier (no type mapping; always falls back to status class).
func (classifier) MapErrorType(string) error { return nil }

// MapSpecial 实现 adapter.Classifier（无特有错误认领）.
// [EN] Implement adapter.Classifier (nothing special to claim).
func (classifier) MapSpecial(error) error { return nil }
