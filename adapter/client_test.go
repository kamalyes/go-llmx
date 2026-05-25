/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-08-11 21:36:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-08-11 21:36:00
 * @FilePath: \go-llmx\adapter\client_test.go
 * @Description: 适配器公共基座测试 —— 构造默认值/选项/访问器/模型解析
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package adapter

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testClient 测试用基座持有者（模拟适配器 Client 内嵌 Base）.
// [EN] Test base holder (mimics an adapter Client embedding Base).
type testClient struct {
	Base
}

// Adapter 实现 HasBase.
// [EN] Implement HasBase.
func (t *testClient) Adapter() *Base { return &t.Base }

// newTestClient 构造测试客户端（模拟适配器 New 的基座装配）.
// [EN] Build a test client (mimics adapter New wiring).
func newTestClient(opts ...Option) *testClient {
	c := &testClient{NewBase("/test", "key-0", "m-0")}
	c.BaseURL = "https://api.example.com"
	Apply(c, opts...)
	return c
}

func TestNewBase_Defaults(t *testing.T) {
	b := NewBase("/chat", "k", "m")
	assert.Equal(t, "k", b.APIKey)
	assert.Equal(t, "m", b.Model)
	assert.Equal(t, "/chat", b.Path)
	// 传输与日志取默认（非 nil，可安全收发）
	require.NotNil(t, b.TC)
	require.NotNil(t, b.Logger)
}

func TestOptions_All(t *testing.T) {
	l := logger.NewEmptyLogger()
	hc := &http.Client{Timeout: 3 * time.Second}
	c := newTestClient(
		WithAPIKey("key-1"),
		WithBaseURL("https://gw.example.com/"),
		WithModel("m-1"),
		WithTimeout(5*time.Second),
		WithHTTPClient(hc),
		WithLogger(l),
	)

	assert.Equal(t, "key-1", c.APIKey)
	// 尾斜杠归一
	assert.Equal(t, "https://gw.example.com", c.BaseURL)
	assert.Equal(t, "m-1", c.Model)
	require.NotNil(t, c.TC)
	require.NotNil(t, c.Logger)
}

func TestOptions_NilIgnored(t *testing.T) {
	// nil http.Client / nil logger 不覆盖默认装配
	c := newTestClient(WithHTTPClient(nil), WithLogger(nil))
	require.NotNil(t, c.TC)
	require.NotNil(t, c.Logger)

	// Apply 跳过 nil 选项
	Apply(c, nil, WithAPIKey("k2"), nil)
	assert.Equal(t, "k2", c.APIKey)
}

func TestAccessors(t *testing.T) {
	c := newTestClient()

	// BaseURL：Get/Set + 尾斜杠归一
	c.SetBaseURL("https://a.example.com///")
	assert.Equal(t, "https://a.example.com", c.GetBaseURL())

	// Model：空串不生效防误清空
	c.SetModel("m-2")
	assert.Equal(t, "m-2", c.GetModel())
	c.SetModel("")
	assert.Equal(t, "m-2", c.GetModel())

	// APIKey：Get/Set（空串允许，支持清空降级）
	c.SetAPIKey("rotated")
	assert.Equal(t, "rotated", c.GetAPIKey())
	c.SetAPIKey("")
	assert.Equal(t, "", c.GetAPIKey())

	// Endpoint：baseURL + path 收口拼接
	assert.Equal(t, "https://a.example.com/test", c.GetEndpoint())
}

func TestResolveModel(t *testing.T) {
	c := newTestClient() // 默认模型 m-0

	// 请求级 nil → 客户端默认
	assert.Equal(t, "m-0", c.ResolveModel(nil))
	// 请求级空模型 → 客户端默认
	assert.Equal(t, "m-0", c.ResolveModel(&llmx.Options{}))
	// 请求级覆盖优先
	assert.Equal(t, "gpt-x", c.ResolveModel(&llmx.Options{Model: "gpt-x"}))
}

// ============================================================================
// 日志联动（ctx → Base.Logger → TC.Logger 全链路贯通）
// ============================================================================

// captureLogger 日志捕获器（嵌入空实现，仅覆盖 InfoContextKV）.
// [EN] Log capturer (embeds the empty impl, overriding InfoContextKV).
type captureLogger struct {
	*logger.EmptyLogger
	captured string
}

func (c *captureLogger) InfoContextKV(_ context.Context, msg string, kv ...interface{}) {
	c.captured = msg + " " + fmt.Sprint(kv...)
}

func TestLogModel(t *testing.T) {
	cl := &captureLogger{EmptyLogger: logger.NewEmptyLogger()}
	c := newTestClient(WithLogger(cl))
	c.LogModel(context.Background(), "m-9", "chat", 3)
	assert.Contains(t, cl.captured, "[LLMX] chat")
	assert.Contains(t, cl.captured, "m-9")
	assert.Contains(t, cl.captured, "3")
}

func TestLoggerLinkedToTransport(t *testing.T) {
	// WithLogger 联动：Base 与 TC 共享同一 logger 实例
	cl := &captureLogger{EmptyLogger: logger.NewEmptyLogger()}
	c := newTestClient(WithLogger(cl))
	assert.Same(t, cl, c.Logger)
	assert.Same(t, cl, c.TC.Logger)

	// WithTimeout/WithHTTPClient 替换 TC 后 logger 不丢
	c2 := newTestClient(WithLogger(cl), WithTimeout(7*time.Second), WithHTTPClient(&http.Client{}))
	assert.Same(t, cl, c2.TC.Logger, "TC 替换后 logger 应保持联动")

	// NewBase 默认装配：TC 与 Base 同源
	b := NewBase("/p", "k", "m")
	assert.Same(t, b.Logger, b.TC.Logger)
}
