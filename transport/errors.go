/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-28 20:27:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-28 20:27:00
 * @FilePath: \go-llmx\transport\errors.go
 * @Description: 传输层错误体系 —— HTTP 状态分类 + 网络错误包装.
 * 核心库哨兵（ErrProviderUnavailable/ErrUnauthorized 等）与 HTTP 语义的桥接层
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package transport

import (
	"errors"
	"fmt"
	"net/http"
)

// networkSentinel 网络层哨兵（DoJSON/DoStream 内部包装目标）.
// [EN] Network-level sentinel (wrap target inside DoJSON/DoStream).
var networkSentinel = errors.New("llmx/transport: network unavailable")

// ErrNetworkUnavailable 网络层错误（DNS/连接拒绝/超时，未收到 HTTP 响应）.
// [EN] Network-level failure (no HTTP response received).
var ErrNetworkUnavailable = networkSentinel

// ErrMarshalRequest 请求体序列化失败（入参结构含 json 不支持的字段类型）.
// [EN] Request body marshaling failed.
var ErrMarshalRequest = errors.New("llmx/transport: marshal request failed")

// ErrUnmarshalResponse 响应体解析失败（2xx 但非法 JSON / 连接中断于传输中途）.
// [EN] Response body unmarshaling failed.
var ErrUnmarshalResponse = errors.New("llmx/transport: unmarshal response failed")

// NewRequestMarshalError 包装序列化错误（统一挂哨兵）.
// [EN] Wrap a marshaling error onto the sentinel.
func NewRequestMarshalError(err error) error {
	return fmt.Errorf("%w: %v", ErrMarshalRequest, err)
}

// NewResponseUnmarshalError 包装解析错误（统一挂哨兵）.
// [EN] Wrap an unmarshaling error onto the sentinel.
func NewResponseUnmarshalError(err error) error {
	return fmt.Errorf("%w: %v", ErrUnmarshalResponse, err)
}

// HTTPStatusClass HTTP 状态语义分类.
// [EN] HTTP status semantic class.
type HTTPStatusClass int

// 状态分类枚举.
// [EN] Status class enum.
const (
	// ClassSuccess 2xx 成功.
	// [EN] 2xx success.
	ClassSuccess HTTPStatusClass = iota

	// ClassInvalidRequest 400/422 请求参数非法.
	// [EN] 400/422 invalid request parameters.
	ClassInvalidRequest

	// ClassUnauthorized 401/403 认证失败.
	// [EN] 401/403 authentication failure.
	ClassUnauthorized

	// ClassNotFound 404 资源不存在（模型名/端点错误）.
	// [EN] 404 resource not found (wrong model or endpoint).
	ClassNotFound

	// ClassRateLimited 429 触发限流.
	// [EN] 429 rate limited.
	ClassRateLimited

	// ClassServerError 5xx 服务端内部错误.
	// [EN] 5xx server internal error.
	ClassServerError

	// ClassUnknown 其它非 2xx 状态.
	// [EN] Other non-2xx status.
	ClassUnknown
)

// ClassifyStatus HTTP 状态码 → 语义分类.
// [EN] Classify an HTTP status code.
func ClassifyStatus(status int) HTTPStatusClass {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ClassInvalidRequest
	case http.StatusUnauthorized, http.StatusForbidden:
		return ClassUnauthorized
	case http.StatusNotFound:
		return ClassNotFound
	case http.StatusTooManyRequests:
		return ClassRateLimited
	default:
		if status >= 200 && status < 300 {
			return ClassSuccess
		}
		if status >= 500 {
			return ClassServerError
		}
		return ClassUnknown
	}
}

// StatusError 携带 HTTP 状态码与响应体的传输错误.
// [EN] Transport error carrying HTTP status and body.
type StatusError struct {
	// Status HTTP 状态码.
	// [EN] HTTP status code.
	Status int

	// Class 状态语义分类.
	// [EN] Status semantic class.
	Class HTTPStatusClass

	// Body 响应体片段（截断至 MaxErrorBodySnippet）.
	// [EN] Response body snippet (truncated).
	Body string
}

// Error 实现 error 接口.
// [EN] Implement the error interface.
func (e *StatusError) Error() string {
	return fmt.Sprintf("llmx/transport: status=%d body=%.200s", e.Status, e.Body)
}

// NewStatusError 构造 StatusError（响应体自动截断）.
// [EN] Build a StatusError with automatic body truncation.
func NewStatusError(status int, body string) *StatusError {
	if len(body) > MaxErrorBodySnippet {
		body = body[:MaxErrorBodySnippet]
	}
	return &StatusError{Status: status, Class: ClassifyStatus(status), Body: body}
}
