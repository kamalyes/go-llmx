/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 20:55:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 20:55:00
 * @FilePath: \go-llmx\adapters\anthropic\errors.go
 * @Description: Anthropic 适配器错误差异 —— wire 错误体解析 + Classifier 实现（协议错误类型
 * 映射/流中 error 事件认领）. 公共映射骨架见核心库 adapter 包
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcanthropic

import (
	"encoding/json"
	"errors"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// wireAPIError Anthropic 协议的错误响应体结构.
// [EN] Error response structure of the Anthropic protocol.
type wireAPIError struct {
	// Type 顶层类型（错误体为 error）.
	// [EN] Top-level type ("error" for error bodies).
	Type string `json:"type"`

	// Error 错误详情容器.
	// [EN] Error detail container.
	Error *wireErrorBody `json:"error,omitempty"`
}

// wireErrorBody 错误详情.
// [EN] Error detail body.
type wireErrorBody struct {
	// Type 错误类型（invalid_request_error / rate_limit_error / overloaded_error 等）.
	// [EN] Error type.
	Type string `json:"type"`

	// Message 人类可读错误信息.
	// [EN] Human-readable error message.
	Message string `json:"message"`
}

// anthropicClassifier Anthropic 协议的错误差异注入.
// [EN] Anthropic error classification.
type anthropicClassifier struct{}

// ParseErrorBody 实现 adapter.Classifier（非 JSON 回退原始文本）.
// [EN] Implement adapter.Classifier (raw text fallback).
func (anthropicClassifier) ParseErrorBody(body string) *adapter.ErrorBody {
	var we wireAPIError
	if err := json.Unmarshal([]byte(body), &we); err != nil || we.Error == nil {
		return &adapter.ErrorBody{Message: body, Type: adapter.ErrorTypeHTTP}
	}
	return &adapter.ErrorBody{Type: we.Error.Type, Message: we.Error.Message}
}

// MapErrorType 实现 adapter.Classifier（协议错误类型 → llmx 哨兵；overloaded 视为限流语义）.
// [EN] Implement adapter.Classifier (protocol error type to sentinel).
func (anthropicClassifier) MapErrorType(errType string) error {
	return mapSentinel(errType)
}

// MapSpecial 实现 adapter.Classifier（认领流中 error 事件）.
// [EN] Implement adapter.Classifier (claim mid-stream error events).
func (anthropicClassifier) MapSpecial(err error) error {
	var serr *streamError
	if errors.As(err, &serr) && serr.body != nil {
		return mapErrorBody(serr.body.Type, serr.body.Type+": "+serr.body.Message)
	}
	return nil
}

// mapSentinel 协议错误类型 → llmx 哨兵（未识别返回 nil 走状态分类兜底）.
// [EN] Protocol error type to sentinel (nil falls back to status class).
func mapSentinel(errType string) error {
	switch errType {
	case "authentication_error", "permission_error":
		return llmx.ErrUnauthorized
	case "rate_limit_error", "overloaded_error":
		return llmx.ErrRateLimited
	case "invalid_request_error", "request_too_large":
		return llmx.ErrInvalidRequest
	case "not_found_error":
		return llmx.ErrProviderUnavailable
	case "api_error":
		return llmx.ErrAPIServerError
	default:
		return nil
	}
}

// mapErrorBody 错误类型/载荷 → 哨兵错误（GenerateContent 200-error 与流中 error 共用出口；
// 未识别类型按服务端错误兜底）.
// [EN] Map an error type/payload onto a sentinel (unified exit; unknown types default).
func mapErrorBody(errType, wrapped string) error {
	if sentinel := mapSentinel(errType); sentinel != nil {
		return fmt.Errorf("%w: %s", sentinel, wrapped)
	}
	return fmt.Errorf("%w: %s", llmx.ErrAPIServerError, wrapped)
}
