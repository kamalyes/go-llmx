/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 21:29:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 21:29:00
 * @FilePath: \go-llmx\adapters\anthropic\errors_test.go
 * @Description: Anthropic 适配器错误映射测试 —— 协议错误类型/状态分类/非 JSON 体/
 * 网络层/流中 error 事件/200 带 error 字段. mock 基建见 anthropic_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcanthropic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
	"github.com/stretchr/testify/assert"
)

func TestErrorMapping_Unauthorized(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrUnauthorized)
}

func TestErrorMapping_RateLimited(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrRateLimited)
}

func TestErrorMapping_InvalidRequest(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"bad param"}}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestErrorMapping_NonJSONBody(t *testing.T) {
	// 非 JSON 错误体 → 按状态分类兜底（502 → 服务端错误）
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		fmt.Fprint(w, "Bad Gateway")
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
}

func TestErrorMapping_NetworkRefused(t *testing.T) {
	c := New("k", WithBaseURL("http://127.0.0.1:1"))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)
}

func TestErrorMapping_200WithErrorField(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"type":"error","error":{"type":"api_error","message":"quota exceeded"}}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
}

func TestStreamErrorEvent(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	// 流中 error 事件 → overloaded 视为限流语义
	assert.ErrorIs(t, err, llmx.ErrRateLimited)
}

func TestMapTransportError_Cases(t *testing.T) {
	cls := anthropicClassifier{}

	// nil / 未知透传 / 停止信号
	assert.NoError(t, adapter.MapTransportError(nil, cls))
	sentinel := errors.New("custom")
	assert.Same(t, sentinel, adapter.MapTransportError(sentinel, cls))
	assert.Equal(t, llmx.ErrStreamClosed, adapter.MapTransportError(llmx.ErrStopStream, cls))

	// 200 错误体字符串 → 按协议类型映射
	raw := errors.New(`{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`)
	err := adapter.MapTransportError(raw, cls)
	assert.ErrorIs(t, err, llmx.ErrRateLimited)
}

func TestMapSentinel_UnknownTypeNil(t *testing.T) {
	// 未识别协议错误类型 → nil（走状态分类兜底）
	assert.Nil(t, mapSentinel("weird_error"))
	// api_error → 服务端错误
	assert.ErrorIs(t, mapSentinel("api_error"), llmx.ErrAPIServerError)
}

func TestMapSentinel_AllBranches(t *testing.T) {
	cases := []struct {
		errType string
		want    error
	}{
		{"authentication_error", llmx.ErrUnauthorized},
		{"permission_error", llmx.ErrUnauthorized},
		{"rate_limit_error", llmx.ErrRateLimited},
		{"overloaded_error", llmx.ErrRateLimited},
		{"invalid_request_error", llmx.ErrInvalidRequest},
		{"request_too_large", llmx.ErrInvalidRequest},
		{"not_found_error", llmx.ErrProviderUnavailable},
	}
	for _, tc := range cases {
		assert.ErrorIs(t, mapSentinel(tc.errType), tc.want, "errType=%s", tc.errType)
	}
}

func TestStreamError_ErrorString(t *testing.T) {
	// 正常载荷：type: message
	e := &streamError{body: &wireErrorBody{Type: "overloaded_error", Message: "busy"}}
	assert.Equal(t, "overloaded_error: busy", e.Error())

	// nil 载荷兜底类型字面量
	assert.Equal(t, errorTypeAnthropic, (&streamError{}).Error())
}

func TestMapErrorBody_UnknownFallback(t *testing.T) {
	// 未识别类型按服务端错误兜底
	err := mapErrorBody("weird_error", "some wrapped text")
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
}
