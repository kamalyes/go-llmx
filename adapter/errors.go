/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-08-11 21:09:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-08-11 21:09:00
 * @FilePath: \go-llmx\adapter\errors.go
 * @Description: 适配器公共错误映射 —— transport 错误 → llmx 哨兵的通用骨架.
 * 公共判定（流终止/网络/状态分类）收口在此；协议差异（wire 错误体解析/
 * 错误类型映射/特有错误认领）由各适配器以 Classifier 注入
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package adapter

import (
	"errors"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/transport"
)

// ErrorTypeHTTP 非 JSON 响应体回退时的错误类型标记.
// [EN] Fallback error type for non-JSON bodies.
const ErrorTypeHTTP = "http_error"

// ErrorBody 协议错误载荷（各适配器解析 wire 错误体后归一）.
// [EN] Protocol error payload (normalized after wire parsing).
type ErrorBody struct {
	// Type 协议错误类型（非 JSON 回退 ErrorTypeHTTP）.
	// [EN] Protocol error type (ErrorTypeHTTP fallback).
	Type string

	// Message 人类可读错误信息.
	// [EN] Human-readable message.
	Message string
}

// Classifier 适配器错误差异注入.
// [EN] Adapter-specific error classification.
type Classifier interface {
	// ParseErrorBody 解析响应体为错误载荷（非 JSON 回退原始文本 + ErrorTypeHTTP）.
	// [EN] Parse a body into an error payload (raw text fallback).
	ParseErrorBody(body string) *ErrorBody

	// MapErrorType 协议错误类型 → llmx 哨兵（未识别返回 nil，走状态分类兜底）.
	// [EN] Map a protocol error type to a sentinel (nil falls back to status class).
	MapErrorType(errType string) error

	// MapSpecial 适配器特有错误认领（如流中 error 事件；不认领返回 nil）.
	// [EN] Claim adapter-specific errors (e.g. mid-stream error events; nil if not claimed).
	MapSpecial(err error) error
}

// MapTransportError transport 错误 → llmx 哨兵错误（适配器统一出口）.
// [EN] Map transport errors to llmx sentinels (unified adapter exit).
//
// 判定顺序：nil → handler 主动终止 → 网络层 → 适配器特有 → HTTP 状态
// （协议错误类型优先，未识别按状态分类兜底）→ 200 带 error 字段 → 原样透传
func MapTransportError(err error, cls Classifier) error {
	if err == nil {
		return nil
	}

	// handler 主动终止 → 流关闭哨兵（正常语义呈现，非 provider 故障）
	if errors.Is(err, llmx.ErrStopStream) {
		return llmx.ErrStreamClosed
	}

	// 网络层错误 → provider 不可达（%w 保留内层 ctx 取消等语义链）
	// [EN] Network error → provider unavailable (inner chain preserved for ctx cancellation).
	if errors.Is(err, transport.ErrNetworkUnavailable) {
		return fmt.Errorf("%w: %w", llmx.ErrProviderUnavailable, err)
	}

	// 适配器特有错误（如流中 error 事件）
	if special := cls.MapSpecial(err); special != nil {
		return special
	}

	// HTTP 状态错误 → 协议错误类型优先，未识别按状态分类兜底
	var se *transport.StatusError
	if errors.As(err, &se) {
		body := cls.ParseErrorBody(se.Body)
		if body.Type != ErrorTypeHTTP && body.Type != "" {
			if sentinel := cls.MapErrorType(body.Type); sentinel != nil {
				return fmt.Errorf("%w: %s: %s", sentinel, body.Type, body.Message)
			}
		}
		return fmt.Errorf("%w: %s: %s", classToSentinel(se.Class), body.Type, body.Message)
	}

	// 响应体内的 error 字段（部分网关 200 也带错误）
	if body := cls.ParseErrorBody(err.Error()); body.Type != ErrorTypeHTTP && body.Message != "" {
		if sentinel := cls.MapErrorType(body.Type); sentinel != nil {
			return fmt.Errorf("%w: %s: %s", sentinel, body.Type, body.Message)
		}
		return fmt.Errorf("%s: %s", body.Type, body.Message)
	}
	return err
}

// WrapErrorBody 协议错误载荷 → 哨兵错误（GenerateContent 200-error 路径共用）.
// [EN] Wrap a wire error payload onto a sentinel (200-with-error path).
func WrapErrorBody(body *ErrorBody) error {
	if body == nil {
		return llmx.ErrAPIServerError
	}
	return fmt.Errorf("%w: %s: %s", llmx.ErrAPIServerError, body.Type, body.Message)
}

// classToSentinel HTTP 状态分类 → llmx 哨兵（两套适配器语义一致的公共映射）.
// [EN] HTTP status class to llmx sentinel (shared mapping).
func classToSentinel(class transport.HTTPStatusClass) error {
	switch class {
	case transport.ClassUnauthorized:
		return llmx.ErrUnauthorized
	case transport.ClassRateLimited:
		return llmx.ErrRateLimited
	case transport.ClassInvalidRequest:
		return llmx.ErrInvalidRequest
	case transport.ClassNotFound:
		return llmx.ErrProviderUnavailable
	case transport.ClassServerError:
		return llmx.ErrAPIServerError
	default:
		return llmx.ErrProviderUnavailable
	}
}
