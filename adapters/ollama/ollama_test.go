/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 21:32:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-25 21:38:00
 * @FilePath: \go-llmx\adapters\ollama\ollama_test.go
 * @Description: Ollama 适配器编排层测试 —— 客户端生命周期/访问器/请求组装/基础收发.
 * mock 基建在本文件维护，供 wire/stream/errors 测试共享；协议编解码见 wire_test.go，
 * 流式聚合见 stream_test.go，错误映射见 errors_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollama

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

// newMockServer 构造 /api/chat 模拟端点.
// [EN] Build a mock /api/chat endpoint.
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
			"model": "llama3.2",
			"message": {"role": "assistant", "content": "你好，世界"},
			"done": true,
			"done_reason": "stop",
			"prompt_eval_count": 10,
			"eval_count": 5
		}`)
	})

	c := New("test-key", WithBaseURL(m.srv.URL), WithModel("llama3.2"))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("hi")})
	require.NoError(t, err)

	// 响应解码
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "你好，世界", resp.Choices[0].Text())
	assert.Equal(t, "stop", resp.Choices[0].FinishReason)
	// 协议差异：无 total 字段，本地求和
	assert.Equal(t, 10, resp.Usage.PromptTokens)
	assert.Equal(t, 5, resp.Usage.CompletionTokens)
	assert.Equal(t, 15, resp.Usage.TotalTokens)

	// 请求体断言
	assert.Equal(t, "llama3.2", m.body("model"))
	assert.Equal(t, false, m.body("stream"))
	msgs := m.body("messages").([]any)
	first := msgs[0].(map[string]any)
	assert.Equal(t, "user", first["role"])
	assert.Equal(t, "hi", first["content"])
}

func TestGenerateContent_StreamNilHandler(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"message": {"role": "assistant", "content": "直通"},
			"done": true,
			"done_reason": "stop"
		}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	// stream 为 nil → 退化为非流式
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, nil)
	require.NoError(t, err)
	assert.Equal(t, "直通", resp.Choices[0].Text())
}

// ============================================================================
// 请求选项编码（buildRequest 消费链路）
// ============================================================================

func TestBuildRequestMoreOptions(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": "ok"}, "done": true, "done_reason": "stop"}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")},
		llmx.WithTemperature(0.7),
		llmx.WithTopP(0.9),
		llmx.WithMaxTokens(256),
		llmx.WithSeed(42),
		llmx.WithStop("END"),
		llmx.WithJSONMode(),
		llmx.WithModel("qwen3"),
	)
	require.NoError(t, err)

	// 协议差异：采样参数收拢在 options 容器（num_predict 对应 max_tokens）
	opts := m.body("options").(map[string]any)
	assert.Equal(t, 0.7, opts["temperature"])
	assert.Equal(t, 0.9, opts["top_p"])
	assert.Equal(t, float64(256), opts["num_predict"])
	assert.Equal(t, float64(42), opts["seed"])
	assert.Equal(t, "END", opts["stop"].([]any)[0])
	// 协议差异：JSON 强制输出走顶层 format 字段
	assert.Equal(t, "json", m.body("format"))
	// 请求级模型覆盖
	assert.Equal(t, "qwen3", m.body("model"))
}

func TestBuildRequestTools(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": "ok"}, "done": true, "done_reason": "stop"}`)
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
	assert.Equal(t, "function", tw["type"])
	fn := tw["function"].(map[string]any)
	assert.Equal(t, "weather", fn["name"])
	assert.Equal(t, "查天气", fn["description"])
	assert.NotNil(t, fn["parameters"])
}

// ============================================================================
// 访问器（Get/Set 双通道）与构造选项
// ============================================================================

func TestAccessorDefaults(t *testing.T) {
	c := New("my-key")
	assert.Equal(t, DefaultBaseURL, c.GetBaseURL())
	assert.Equal(t, DefaultModel, c.GetModel())
	assert.Equal(t, "my-key", c.GetAPIKey())
	assert.Equal(t, DefaultBaseURL+ChatPath, c.GetEndpoint())
}

func TestAccessorRuntimeSwitch(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": "ok"}, "done": true, "done_reason": "stop"}`)
	})

	c := New("k")
	c.SetBaseURL(m.srv.URL + "/")
	assert.Equal(t, m.srv.URL, c.GetBaseURL())
	assert.Equal(t, m.srv.URL+ChatPath, c.GetEndpoint())

	c.SetModel("qwen3:8b")
	assert.Equal(t, "qwen3:8b", c.GetModel())
	// SetModel 空串不生效（防误清空）
	c.SetModel("")
	assert.Equal(t, "qwen3:8b", c.GetModel())

	c.SetAPIKey("rotated")
	assert.Equal(t, "rotated", c.GetAPIKey())

	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	// 热更后的模型生效
	assert.Equal(t, "qwen3:8b", m.body("model"))
}

func TestAccessorWithOverrides(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": "ok"}, "done": true, "done_reason": "stop"}`)
	})

	c := New("k",
		WithBaseURL(m.srv.URL),
		WithAPIKey("with-key"),
		WithModel("qwen3:8b"),
		WithLogger(logger.NewEmptyLogger()),
	)
	assert.Equal(t, "with-key", c.GetAPIKey())
	assert.Equal(t, "qwen3:8b", c.GetModel())

	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
}

func TestAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": "ok"}, "done": true, "done_reason": "stop"}`)
	}))
	defer srv.Close()

	// 本地部署：空 key 不带认证头
	c := New("", WithBaseURL(srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	assert.Empty(t, gotAuth)

	// 代理网关：Bearer 形态
	c2 := New("secret", WithBaseURL(srv.URL))
	_, err = c2.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	assert.Equal(t, "Bearer secret", gotAuth)
}
