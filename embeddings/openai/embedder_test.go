/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 22:50:00
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-17 11:02:36
 * @FilePath: \go-llmx\embeddings\openai\embedder_test.go
 * @Description: OpenAI 兼容嵌入适配器测试 —— 批量/单条/index 归位/向量数校验/
 * 错误映射/访问器. mock 基建独立维护（子包不依赖对话测试）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcembed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// mock 基建
// ============================================================================

// mockServer 捕获最近请求体的模拟端点.
// [EN] Mock endpoint capturing the last request body.
type mockServer struct {
	mu       sync.Mutex
	lastBody map[string]any
	srv      *httptest.Server
}

// newMockServer 构造 /embeddings 模拟端点.
// [EN] Build a mock /embeddings endpoint.
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

// embeddingsResponder 构造按输入数量回放的向量响应（每条 3 维递增向量）.
// [EN] Build a vector response replayed by input count.
func embeddingsResponder(n int) string {
	out := `{"data": [`
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf(`{"index": %d, "embedding": [%d.0, 0.5, -0.25]}`, i, i)
	}
	return out + `]}`
}

// ============================================================================
// 嵌入链路
// ============================================================================

func TestEmbedDocuments(t *testing.T) {
	texts := []string{"你好", "hello", "Bonjour"}
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, embeddingsResponder(3))
	})

	c := New("k", WithBaseURL(m.srv.URL))
	vectors, err := c.EmbedDocuments(context.Background(), texts)
	require.NoError(t, err)

	require.Len(t, vectors, 3)
	// 响应按 index 归位
	assert.Equal(t, []float64{0, 0.5, -0.25}, vectors[0])
	assert.Equal(t, []float64{1, 0.5, -0.25}, vectors[1])
	assert.Equal(t, []float64{2, 0.5, -0.25}, vectors[2])

	// 请求体断言
	assert.Equal(t, DefaultModel, m.body("model"))
	input := m.body("input").([]any)
	require.Len(t, input, 3)
	assert.Equal(t, "你好", input[0])
}

func TestEmbedQuery(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data": [{"index": 0, "embedding": [0.1, 0.2]}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	vec, err := c.EmbedQuery(context.Background(), "查询")
	require.NoError(t, err)
	assert.Equal(t, []float64{0.1, 0.2}, vec)

	// 单条嵌入以单元素数组形态发送
	input := m.body("input").([]any)
	require.Len(t, input, 1)
	assert.Equal(t, "查询", input[0])
}

func TestEmbed_IndexReordering(t *testing.T) {
	// 响应乱序返回（index 2/0/1）→ 按 index 归位到输入顺序
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data": [
			{"index": 2, "embedding": [3.0]},
			{"index": 0, "embedding": [1.0]},
			{"index": 1, "embedding": [2.0]}
		]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	vectors, err := c.EmbedDocuments(context.Background(), []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Equal(t, []float64{1.0}, vectors[0])
	assert.Equal(t, []float64{2.0}, vectors[1])
	assert.Equal(t, []float64{3.0}, vectors[2])
}

func TestEmbed_VectorCountMismatch(t *testing.T) {
	// 响应向量数与输入不匹配（网关异常载荷）→ 统一数量校验拦截
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data": [
			{"index": 0, "embedding": [1.0]},
			{"index": 9, "embedding": [9.9]}
		]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.EmbedDocuments(context.Background(), []string{"a"})
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
}

func TestEmbed_EmptyBatch(t *testing.T) {
	// 空批量前置拦截（不出网）
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("空批量不应发出请求")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.EmbedDocuments(context.Background(), []string{})
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// ============================================================================
// 访问器（Get/Set 双通道）与构造选项
// ============================================================================

func TestAccessorDefaults(t *testing.T) {
	c := New("my-key")
	assert.Equal(t, DefaultBaseURL, c.GetBaseURL())
	assert.Equal(t, DefaultModel, c.GetModel())
	assert.Equal(t, "my-key", c.GetAPIKey())
	assert.Equal(t, DefaultBaseURL+EmbeddingsPath, c.GetEndpoint())
}

func TestAccessorRuntimeSwitch(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data": [{"index": 0, "embedding": [0.1]}]}`)
	})

	c := New("k")
	c.SetBaseURL(m.srv.URL + "/")
	assert.Equal(t, m.srv.URL, c.GetBaseURL())
	assert.Equal(t, m.srv.URL+EmbeddingsPath, c.GetEndpoint())

	c.SetModel("text-embedding-3-large")
	assert.Equal(t, "text-embedding-3-large", c.GetModel())
	// SetModel 空串不生效（防误清空）
	c.SetModel("")
	assert.Equal(t, "text-embedding-3-large", c.GetModel())

	c.SetAPIKey("rotated")
	assert.Equal(t, "rotated", c.GetAPIKey())

	_, err := c.EmbedQuery(context.Background(), "q")
	require.NoError(t, err)
	// 热更后的模型生效
	assert.Equal(t, "text-embedding-3-large", m.body("model"))
}

func TestAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"data": [{"index": 0, "embedding": [0.1]}]}`)
	}))
	defer srv.Close()

	c := New("secret", WithBaseURL(srv.URL))
	_, err := c.EmbedQuery(context.Background(), "q")
	require.NoError(t, err)
	assert.Equal(t, "Bearer secret", gotAuth)
}

// ============================================================================
// 错误映射
// ============================================================================

func TestEmbedError_StatusClasses(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{400, llmx.ErrInvalidRequest},
		{401, llmx.ErrUnauthorized},
		{429, llmx.ErrRateLimited},
		{500, llmx.ErrAPIServerError},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, `{"error": {"message": "boom", "type": "api_error"}}`)
			})
			c := New("k", WithBaseURL(m.srv.URL))
			_, err := c.EmbedQuery(context.Background(), "q")
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestEmbedError_NonJSONBody(t *testing.T) {
	// 非 JSON 错误体 → 按状态分类兜底（502 → 服务端错误）
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		fmt.Fprint(w, "Bad Gateway")
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.EmbedQuery(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
}

func TestEmbedError_NetworkRefused(t *testing.T) {
	c := New("k", WithBaseURL("http://127.0.0.1:1"))
	_, err := c.EmbedQuery(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)
}

func TestEmbedError_200WithErrorField(t *testing.T) {
	// 部分网关 200 状态仍注入 error 字段（配额耗尽等）
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"error": {"message": "quota exceeded", "type": "insufficient_quota"}}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.EmbedQuery(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrAPIServerError)
}

func TestMapTransportError_Cases(t *testing.T) {
	cls := classifier{}

	// nil 透传
	assert.NoError(t, adapter.MapTransportError(nil, cls))

	// 未知错误 → 原样透传
	sentinel := fmt.Errorf("custom")
	assert.Equal(t, sentinel, adapter.MapTransportError(sentinel, cls))
}

func TestParseErrorBody(t *testing.T) {
	cls := classifier{}

	// 协议错误体 → 归一载荷
	body := cls.ParseErrorBody(`{"error": {"message": "bad", "type": "invalid_request_error"}}`)
	require.NotNil(t, body)
	assert.Equal(t, "invalid_request_error", body.Type)
	assert.Equal(t, "bad", body.Message)

	// 非 JSON → 回退原文 + http_error 标记
	fallback := cls.ParseErrorBody("raw text")
	assert.Equal(t, adapter.ErrorTypeHTTP, fallback.Type)
	assert.Equal(t, "raw text", fallback.Message)

	// JSON 但无 error 字段 → 回退
	missing := cls.ParseErrorBody(`{"ok": true}`)
	assert.Equal(t, adapter.ErrorTypeHTTP, missing.Type)
}

func TestClassifier_EmptyImplementations(t *testing.T) {
	cls := classifier{}
	// 无类型映射与特有认领 → 恒 nil
	assert.NoError(t, cls.MapErrorType("any"))
	assert.NoError(t, cls.MapSpecial(fmt.Errorf("any")))
}
