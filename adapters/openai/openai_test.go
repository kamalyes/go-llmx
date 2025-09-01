/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-01 21:26:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-01 21:26:00
 * @FilePath: \go-llmx\adapters\openai\openai_test.go
 * @Description: OpenAI 适配器编排层测试 —— 客户端生命周期/访问器/请求组装/基础收发.
 * mock 基建在本文件维护，供 wire/stream/errors 测试共享；协议编解码见 wire_test.go，
 * 流式聚合见 stream_test.go，错误映射见 errors_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	llmx "github.com/kamalyes/go-llmx"
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

// newMockServer 构造 /chat/completions 模拟端点.
// [EN] Build a mock /chat/completions endpoint.
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
// 基础收发（非流式 / 流式 / 空响应）
// ============================================================================

func TestGenerateContent_NonStream(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"choices": [{
				"message": {"role": "assistant", "content": "你好，世界"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`)
	})

	c := New("test-key", WithBaseURL(m.srv.URL), WithModel("gpt-4o-mini"))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("hi")})
	require.NoError(t, err)

	text, err := llmx.FirstText(resp)
	require.NoError(t, err)
	assert.Equal(t, "你好，世界", text)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
	assert.Equal(t, "gpt-4o-mini", resp.Model)
	// 请求体断言
	assert.Equal(t, "gpt-4o-mini", m.body("model"))
	msgs := m.body("messages").([]any)
	first := msgs[0].(map[string]any)
	assert.Equal(t, "user", first["role"])
	assert.Equal(t, "hi", first["content"])
}

func TestGenerateContent_Stream(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"你"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"好"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	var collected string
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("hi")}, llmx.StreamCollector(&collected))
	require.NoError(t, err)

	assert.Equal(t, "你好", collected)
	assert.Equal(t, "你好", resp.Choices[0].Text())
	assert.Equal(t, "stop", resp.Choices[0].FinishReason)
	assert.Equal(t, 3, resp.Usage.TotalTokens)
	assert.True(t, m.body("stream").(bool))
}

func TestEmptyChoicesError(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": []}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
}

// ============================================================================
// 请求选项编码（buildRequest 消费链路）
// ============================================================================

func TestOptionsEncoding(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")},
		llmx.WithTemperature(0.2),
		llmx.WithMaxTokens(100),
		llmx.WithStop("END"),
		llmx.WithJSONMode(),
		llmx.WithModel("deepseek-chat"),
		llmx.WithTools(llmx.ToolDef{
			Name:        "search",
			Description: "搜索",
			Parameters:  map[string]any{"type": "object"},
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, "deepseek-chat", m.body("model"))
	assert.Equal(t, 0.2, m.body("temperature"))
	assert.Equal(t, float64(100), m.body("max_tokens"))
	assert.Equal(t, "json_object", m.body("response_format").(map[string]any)["type"])
	tools := m.body("tools").([]any)
	require.Len(t, tools, 1)
	fn := tools[0].(map[string]any)["function"].(map[string]any)
	assert.Equal(t, "search", fn["name"])
}

func TestBuildRequestMoreOptions(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")},
		llmx.WithTopP(0.9),
		llmx.WithN(3),
		llmx.WithSeed(42),
	)
	require.NoError(t, err)

	assert.Equal(t, 0.9, m.body("top_p"))
	assert.Equal(t, float64(3), m.body("n"))
	assert.Equal(t, float64(42), m.body("seed"))
}

// ============================================================================
// 访问器（Get/Set 双通道）与构造选项
// ============================================================================

func TestAccessorDefaults(t *testing.T) {
	c := New("my-key")
	assert.Equal(t, DefaultBaseURL, c.GetBaseURL())
	assert.Equal(t, DefaultModel, c.GetModel())
	assert.Equal(t, "my-key", c.GetAPIKey())
	assert.Equal(t, DefaultBaseURL+ChatCompletionsPath, c.GetEndpoint())
}

func TestAccessorRuntimeSwitch(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}`)
	})

	c := New("k")
	// SetBaseURL 尾斜杠归一 + 请求打到新端点
	c.SetBaseURL(m.srv.URL + "/")
	assert.Equal(t, m.srv.URL, c.GetBaseURL())
	assert.Equal(t, m.srv.URL+ChatCompletionsPath, c.GetEndpoint())

	c.SetModel("deepseek-chat")
	assert.Equal(t, "deepseek-chat", c.GetModel())
	// SetModel 空串不生效（防误清空）
	c.SetModel("")
	assert.Equal(t, "deepseek-chat", c.GetModel())

	c.SetAPIKey("rotated")
	assert.Equal(t, "rotated", c.GetAPIKey())

	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	// 热更后的模型生效
	assert.Equal(t, "deepseek-chat", m.body("model"))
}

func TestAccessorWithOverrides(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}`)
	})

	c := New("k",
		WithBaseURL(m.srv.URL),
		WithAPIKey("with-key"),
		WithModel("gpt-4o"),
		WithTimeout(30*time.Second),
		WithHTTPClient(&http.Client{Timeout: time.Second}),
	)
	assert.Equal(t, "with-key", c.GetAPIKey())
	assert.Equal(t, "gpt-4o", c.GetModel())

	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
}
