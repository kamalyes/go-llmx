/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-08-11 21:21:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-08-11 21:21:00
 * @FilePath: \go-llmx\adapter\errors_test.go
 * @Description: 适配器公共错误映射测试 —— 判定顺序/状态分类兜底/
 * 200 带 error 字段路径/WrapErrorBody
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package adapter

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/transport"
	"github.com/stretchr/testify/assert"
)

// testClassifier 可配置桩（"type|message" 管道格式解析，覆盖全部分支）.
// [EN] Configurable stub (parses "type|message" pipes, covers all branches).
type testClassifier struct {
	// mapType 错误类型 → 哨兵映射（nil 表示未识别）.
	// [EN] Type-to-sentinel mapping (nil = unrecognized).
	mapType map[string]error

	// special 特有错误认领（nil 表示不认领）.
	// [EN] Special error claim (nil = not claimed).
	special error

	// claim 输入是否命中特有认领的判定函数.
	// [EN] Predicate deciding whether the special claim applies.
	claim func(err error) bool
}

// ParseErrorBody 实现 Classifier（"type|message" 管道格式；无管道回退原文）.
// [EN] Implement Classifier ("type|message" pipe; raw text fallback).
func (c testClassifier) ParseErrorBody(body string) *ErrorBody {
	if parts := strings.SplitN(body, "|", 2); len(parts) == 2 {
		return &ErrorBody{Type: parts[0], Message: parts[1]}
	}
	return &ErrorBody{Message: body, Type: ErrorTypeHTTP}
}

// MapErrorType 实现 Classifier.
// [EN] Implement Classifier.
func (c testClassifier) MapErrorType(errType string) error {
	if c.mapType == nil {
		return nil
	}
	return c.mapType[errType]
}

// MapSpecial 实现 Classifier.
// [EN] Implement Classifier.
func (c testClassifier) MapSpecial(err error) error {
	if c.claim != nil && c.claim(err) {
		return c.special
	}
	return nil
}

// ============================================================================
// 判定顺序（nil → 主动终止 → 网络层 → 特有认领）
// ============================================================================

func TestMapTransportError_NilAndStopStream(t *testing.T) {
	cls := testClassifier{}
	assert.NoError(t, MapTransportError(nil, cls))

	// handler 主动终止 → 流关闭哨兵
	assert.ErrorIs(t, MapTransportError(llmx.ErrStopStream, cls), llmx.ErrStreamClosed)
	// 包装后的主动终止同样命中
	wrapped := fmt.Errorf("ctx: %w", llmx.ErrStopStream)
	assert.ErrorIs(t, MapTransportError(wrapped, cls), llmx.ErrStreamClosed)
}

func TestMapTransportError_Network(t *testing.T) {
	// 网络层错误 → provider 不可达（保留底层细节）
	err := fmt.Errorf("dial tcp: %w", transport.ErrNetworkUnavailable)
	mapped := MapTransportError(err, testClassifier{})
	assert.ErrorIs(t, mapped, llmx.ErrProviderUnavailable)
	assert.Contains(t, mapped.Error(), "dial tcp")
}

func TestMapTransportError_SpecialClaim(t *testing.T) {
	// 适配器特有错误（如流中 error 事件）优先于状态分类
	claimed := errors.New("stream error event")
	cls := testClassifier{
		special: llmx.ErrAPIServerError,
		claim:   func(err error) bool { return err == claimed },
	}
	assert.ErrorIs(t, MapTransportError(claimed, cls), llmx.ErrAPIServerError)
}

// ============================================================================
// HTTP 状态路径（协议错误类型优先，未识别按状态分类兜底）
// ============================================================================

func TestMapTransportError_StatusTypeSentinel(t *testing.T) {
	// 协议错误类型识别 → 类型哨兵优先（不落状态分类）
	se := transport.NewStatusError(500, "insufficient_quota|quota exceeded")
	cls := testClassifier{mapType: map[string]error{
		"insufficient_quota": llmx.ErrRateLimited,
	}}
	mapped := MapTransportError(se, cls)
	assert.ErrorIs(t, mapped, llmx.ErrRateLimited)
	assert.Contains(t, mapped.Error(), "quota exceeded")
}

func TestMapTransportError_StatusClassFallback(t *testing.T) {
	// 类型未识别 / 空 / http 回退 → 状态分类兜底（body.Type 仍透出）
	cls := testClassifier{mapType: map[string]error{"known": llmx.ErrRateLimited}}

	// ① MapErrorType 返回 nil（未注册类型）
	assert.ErrorIs(t, MapTransportError(transport.NewStatusError(429, "unknown_type|boom"), cls), llmx.ErrRateLimited)

	// ② body.Type 为 ErrorTypeHTTP（非 JSON 回退标记）
	assert.ErrorIs(t, MapTransportError(transport.NewStatusError(401, "raw text"), cls), llmx.ErrUnauthorized)

	// ③ body.Type 为空
	assert.ErrorIs(t, MapTransportError(transport.NewStatusError(400, "|boom"), cls), llmx.ErrInvalidRequest)
}

func TestMapTransportError_AllClasses(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{400, llmx.ErrInvalidRequest},
		{422, llmx.ErrInvalidRequest},
		{401, llmx.ErrUnauthorized},
		{403, llmx.ErrUnauthorized},
		{404, llmx.ErrProviderUnavailable},
		{429, llmx.ErrRateLimited},
		{500, llmx.ErrAPIServerError},
		{503, llmx.ErrAPIServerError},
	}
	for _, tc := range cases {
		// 构造无类型错误体（恒走状态分类）
		err := transport.NewStatusError(tc.status, "plain failure")
		assert.ErrorIs(t, MapTransportError(err, testClassifier{}), tc.want,
			"status %d", tc.status)
	}
}

func TestMapTransportError_UnknownClass(t *testing.T) {
	// 非常规分类（3xx 等未归类状态）→ 默认哨兵 provider 不可达
	se := &transport.StatusError{Status: 301, Class: transport.ClassUnknown, Body: "redirect"}
	assert.ErrorIs(t, MapTransportError(se, testClassifier{}), llmx.ErrProviderUnavailable)
}

// ============================================================================
// 200 带 error 字段路径（非 StatusError，响应体即错误载体）
// ============================================================================

func TestMapTransportError_200ErrorField(t *testing.T) {
	// 错误串按 "type|message" 解析成功 → 类型哨兵包装
	gatewayErr := errors.New("insufficient_quota|quota exceeded")
	cls := testClassifier{mapType: map[string]error{
		"insufficient_quota": llmx.ErrRateLimited,
	}}
	mapped := MapTransportError(gatewayErr, cls)
	assert.ErrorIs(t, mapped, llmx.ErrRateLimited)
	assert.Contains(t, mapped.Error(), "quota exceeded")
}

func TestMapTransportError_200ErrorFieldNoSentinel(t *testing.T) {
	// 类型未注册哨兵 → 原样格式化透出（type: message）
	gatewayErr := errors.New("gateway_error|upstream down")
	mapped := MapTransportError(gatewayErr, testClassifier{})
	assert.Equal(t, "gateway_error: upstream down", mapped.Error())
}

func TestMapTransportError_PassThrough(t *testing.T) {
	// 无法解析为错误载荷的普通错误 → 原样透传
	plain := errors.New("just a failure")
	assert.Equal(t, plain, MapTransportError(plain, testClassifier{}))
}

// ============================================================================
// WrapErrorBody（GenerateContent 200-error 路径共用）
// ============================================================================

func TestWrapErrorBody(t *testing.T) {
	// nil 载荷 → 纯服务端哨兵
	assert.ErrorIs(t, WrapErrorBody(nil), llmx.ErrAPIServerError)

	// 正常载荷 → 哨兵 + 类型 + 消息
	err := WrapErrorBody(&ErrorBody{Type: "api_error", Message: "boom"})
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
	assert.Contains(t, err.Error(), "api_error: boom")
}
