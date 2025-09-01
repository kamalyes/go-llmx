/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-01 21:15:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-01 21:15:00
 * @FilePath: \go-llmx\adapters\openai\errors_test.go
 * @Description: OpenAI 适配器错误映射测试 —— HTTP 状态分类/非 JSON 体/网络层/
 * 200 带 error 字段/未知透传. mock 基建见 openai_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{401, llmx.ErrUnauthorized},
		{429, llmx.ErrRateLimited},
		{400, llmx.ErrInvalidRequest},
		{500, llmx.ErrAPIServerError},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprintf(w, `{"error": {"message": "boom", "type": "api_error"}}`)
			})
			c := New("k", WithBaseURL(m.srv.URL))
			_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestErrorMapping_NotFound(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"error": {"message": "model not found", "type": "invalid_request_error"}}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)
}

func TestErrorMapping_NonJSONBody(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		fmt.Fprint(w, "Bad Gateway") // 非 JSON 错误体 → 回退原始文本
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
}

func TestErrorMapping_UnknownStatus(t *testing.T) {
	// 418 未分类状态 → default 分支 → ErrProviderUnavailable
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(418)
		fmt.Fprint(w, "I'm a teapot")
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)
}

func TestErrorMapping_NetworkRefused(t *testing.T) {
	// 连接拒绝端口 → 网络哨兵 → ErrProviderUnavailable
	c := New("k", WithBaseURL("http://127.0.0.1:1"))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)
}

func TestErrorMapping_200WithErrorField(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"error": {"message": "quota exceeded", "type": "insufficient_quota"}}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
}

func TestMapTransportError_UnknownPassThrough(t *testing.T) {
	// 非 StatusError、非网络、无 error 字段的未知错误 → 原样透传
	sentinel := errors.New("custom failure")
	assert.Same(t, sentinel, adapter.MapTransportError(sentinel, openaiClassifier{}))
	assert.NoError(t, adapter.MapTransportError(nil, openaiClassifier{}))
	assert.Equal(t, llmx.ErrStreamClosed, adapter.MapTransportError(llmx.ErrStopStream, openaiClassifier{}))
}

func TestMapTransportError_ResponseBodyErrorField(t *testing.T) {
	// 错误字符串本身为 wire error 体 → 命中响应体内 error 字段分支
	raw := errors.New(`{"error": {"message": "m", "type": "t"}}`)
	err := adapter.MapTransportError(raw, openaiClassifier{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "t: m")
}
