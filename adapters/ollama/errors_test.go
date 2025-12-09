/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 21:21:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 21:21:00
 * @FilePath: \go-llmx\adapters\ollama\errors_test.go
 * @Description: Ollama 适配器错误映射测试 —— 状态分类/非 JSON 体/网络层/
 * 200 带 error 字段/流中 error 帧. mock 基建见 ollama_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollama

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

func TestErrorMapping_StatusClasses(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{400, llmx.ErrInvalidRequest},
		{401, llmx.ErrUnauthorized},
		{404, llmx.ErrProviderUnavailable},
		{429, llmx.ErrRateLimited},
		{500, llmx.ErrAPIServerError},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, `{"error": "boom"}`)
			})
			c := New("k", WithBaseURL(m.srv.URL))
			_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
			assert.ErrorIs(t, err, tc.want)
		})
	}
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
	// Ollama 任意状态均可能以 error 字段返回（模型未拉取等）
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"error": "model 'llama3.2' not found, try pulling it first"}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
	assert.Contains(t, err.Error(), "not found")
}

func TestStreamErrorFrame(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, `{"error": "model crashed"}`+"\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	// 流中 error 帧 → 服务端错误
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
	assert.Contains(t, err.Error(), "model crashed")
}

func TestStreamStatusError(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error": "internal"}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
}

func TestMapTransportError_Cases(t *testing.T) {
	cls := ollamaClassifier{}

	// nil / 未知透传 / 停止信号
	assert.NoError(t, adapter.MapTransportError(nil, cls))
	sentinel := errors.New("custom")
	assert.Same(t, sentinel, adapter.MapTransportError(sentinel, cls))
	assert.Equal(t, llmx.ErrStreamClosed, adapter.MapTransportError(llmx.ErrStopStream, cls))

	// 流中 error 帧（*streamError 类型认领）→ 服务端错误
	raw := &streamError{message: "model crashed"}
	err := adapter.MapTransportError(raw, cls)
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)

	// 200 错误体字符串（解析成功无类型映射 → 协议字面量形态透出）
	nonJSON := errors.New(`{"error": "boom"}`)
	err = adapter.MapTransportError(nonJSON, cls)
	assert.EqualError(t, err, "ollama_error: boom")
}

func TestStreamError_ErrorString(t *testing.T) {
	assert.Equal(t, "ollama_error: crashed", (&streamError{message: "crashed"}).Error())
}
