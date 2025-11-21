/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 21:36:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-25 21:31:00
 * @FilePath: \go-llmx\adapters\anthropic\anthropic_test.go
 * @Description: Anthropic 适配器编排层测试 —— 客户端生命周期/访问器/协议头/请求组装/
 * 基础收发. mock 基建在本文件维护，供 wire/stream/errors 测试共享
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcanthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// mock 基建（同包测试共享）
// ============================================================================

// mockServer 按请求路径分发的模拟端点.
// [EN] Mock endpoint dispatching by request path.
type mockServer struct {
	mu       sync.Mutex
	lastBody map[string]any
	srv      *httptest.Server
}

// newMockServer 构造 /v1/messages 模拟端点.
// [EN] Build a mock /v1/messages endpoint.
func newMockServer(t *testing.T, handler http.HandlerFunc) *mockServer {
	m := &mockServer{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		m.lastBody = body
		m.mu.Unlock()
		handler(w, r)
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// body 读取捕获的最近一次请求体字段.
// [EN] Read a field of the last captured request body.
func (m *mockServer) body(key string) any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastBody[key]
}

// ============================================================================
// 基础收发
// ============================================================================

func TestGenerateContent_NonStream(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"id": "msg_1",
			"model": "claude-sonnet-4-5",
			"content": [{"type": "text", "text": "你好，世界"}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`)
	})

	c := New("test-key", WithBaseURL(m.srv.URL), WithModel("claude-sonnet-4-5"))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{
		llmx.System("你是助手"),
		llmx.User("打招呼"),
	})
	require.NoError(t, err)

	// 协议差异：system 走顶层参数（不在 messages 内）
	assert.Equal(t, "你是助手", m.body("system"))
	msgs := m.body("messages").([]any)
	require.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0].(map[string]any)["role"])

	// 响应解码
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "你好，世界", resp.Choices[0].Text())
	assert.Equal(t, "end_turn", resp.Choices[0].FinishReason)
	assert.Equal(t, 10, resp.Usage.PromptTokens)
	assert.Equal(t, 5, resp.Usage.CompletionTokens)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

// ============================================================================
// 请求选项编码（buildRequest 消费链路）
// ============================================================================

func TestBuildRequestMoreOptions(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "ok"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")},
		llmx.WithTemperature(0.7),
		llmx.WithTopP(0.9),
		llmx.WithStop("END"),
		llmx.WithModel("req-model"),
	)
	require.NoError(t, err)

	assert.Equal(t, 0.7, m.body("temperature"))
	assert.Equal(t, 0.9, m.body("top_p"))
	// 停止序列协议字段名 stop_sequences
	stops := m.body("stop_sequences").([]any)
	assert.Equal(t, "END", stops[0])
	// 请求级模型覆盖
	assert.Equal(t, "req-model", m.body("model"))
}

func TestBuildRequestTools(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "ok"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")},
		llmx.WithTools(llmx.ToolDef{
			Name:        "weather",
			Description: "查天气",
			Parameters:  map[string]any{"type": "object"},
		}),
	)
	require.NoError(t, err)

	tools := m.body("tools").([]any)
	require.Len(t, tools, 1)
	tw := tools[0].(map[string]any)
	assert.Equal(t, "weather", tw["name"])
	assert.Equal(t, "查天气", tw["description"])
	// 参数 Schema 协议字段名 input_schema
	assert.NotNil(t, tw["input_schema"])
}

// ============================================================================
// 访问器（Get/Set 双通道）与构造选项
// ============================================================================

func TestAccessorDefaults(t *testing.T) {
	c := New("my-key")
	assert.Equal(t, DefaultBaseURL, c.GetBaseURL())
	assert.Equal(t, DefaultModel, c.GetModel())
	assert.Equal(t, "my-key", c.GetAPIKey())
	assert.Equal(t, DefaultBaseURL+MessagesPath, c.GetEndpoint())
}

func TestAccessorRuntimeSwitch(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "ok"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k")
	c.SetBaseURL(m.srv.URL + "/")
	assert.Equal(t, m.srv.URL, c.GetBaseURL())
	assert.Equal(t, m.srv.URL+MessagesPath, c.GetEndpoint())

	c.SetModel("claude-opus-4-1")
	assert.Equal(t, "claude-opus-4-1", c.GetModel())
	// SetModel 空串不生效（防误清空）
	c.SetModel("")
	assert.Equal(t, "claude-opus-4-1", c.GetModel())

	c.SetAPIKey("rotated")
	assert.Equal(t, "rotated", c.GetAPIKey())

	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	// 热更后的模型生效
	assert.Equal(t, "claude-opus-4-1", m.body("model"))
}

func TestAccessorWithOverrides(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "ok"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k",
		WithBaseURL(m.srv.URL),
		WithAPIKey("with-key"),
		WithModel("claude-haiku-4-5"),
		WithLogger(logger.NewEmptyLogger()),
	)
	assert.Equal(t, "with-key", c.GetAPIKey())
	assert.Equal(t, "claude-haiku-4-5", c.GetModel())

	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
}

func TestHeaders(t *testing.T) {
	var gotKey, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "ok"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	}))
	defer srv.Close()

	c := New("sk-ant-test", WithBaseURL(srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	// 协议差异：x-api-key + anthropic-version（非 Bearer）
	assert.Equal(t, "sk-ant-test", gotKey)
	assert.Equal(t, APIVersion, gotVersion)
}
