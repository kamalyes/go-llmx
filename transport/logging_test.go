/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-25 20:12:07
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-25 20:12:07
 * @FilePath: \go-llmx\transport\logging_test.go
 * @Description: 传输层日志打点测试 —— ContextKV 全链路贯通验证.
 * captureLogger 嵌入 EmptyLogger 仅捕获用到的四个 ContextKV 方法
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package transport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kamalyes/go-logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogger 日志捕获器（嵌入空实现，仅覆盖 ContextKV 四方法）.
// [EN] Log capturer (embeds the empty impl, overriding the four ContextKV methods).
type captureLogger struct {
	*logger.EmptyLogger
	mu      sync.Mutex
	entries []string // "level|msg|kv..."
}

func newCaptureLogger() *captureLogger {
	return &captureLogger{EmptyLogger: logger.NewEmptyLogger()}
}

func (c *captureLogger) record(level, msg string, kv ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, level+"|"+msg+"|"+strings.TrimSuffix(fmtKV(kv...), " "))
}

func fmtKV(kv ...interface{}) string {
	var b strings.Builder
	for i := 0; i+1 < len(kv); i += 2 {
		b.WriteString(strings.TrimSpace(toStr(kv[i])) + "=" + strings.TrimSpace(toStr(kv[i+1])) + " ")
	}
	return b.String()
}

func toStr(v interface{}) string {
	return fmt.Sprint(v)
}

func (c *captureLogger) DebugContextKV(_ context.Context, msg string, kv ...interface{}) { c.record("debug", msg, kv...) }
func (c *captureLogger) InfoContextKV(_ context.Context, msg string, kv ...interface{})  { c.record("info", msg, kv...) }
func (c *captureLogger) WarnContextKV(_ context.Context, msg string, kv ...interface{})  { c.record("warn", msg, kv...) }
func (c *captureLogger) ErrorContextKV(_ context.Context, msg string, kv ...interface{}) { c.record("error", msg, kv...) }

// logs 读取已捕获条目快照.
// [EN] Snapshot the captured entries.
func (c *captureLogger) logs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.entries...)
}

func TestLogging_SuccessAndFailures(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ok.Close()

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()

	// 成功路径：debug 打点含 method/url/status/elapsed
	cl := newCaptureLogger()
	c := NewClient(WithLogger(cl))
	var out map[string]any
	require.NoError(t, c.DoJSON(context.Background(), MethodPost, ok.URL, map[string]any{"q": 1}, &out, nil))
	entries := cl.logs()
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0], "debug|[LLMX] http 请求完成")
	assert.Contains(t, entries[0], "method=POST")
	assert.Contains(t, entries[0], "url="+ok.URL)
	assert.Contains(t, entries[0], "status=200")
	assert.Contains(t, entries[0], "elapsed=")

	// 状态错误路径：warn 打点携带状态码
	cl2 := newCaptureLogger()
	c2 := NewClient(WithLogger(cl2))
	_ = c2.DoJSON(context.Background(), MethodPost, bad.URL, map[string]any{"q": 1}, nil, nil)
	entries = cl2.logs()
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0], "warn|[LLMX] http 状态错误")
	assert.Contains(t, entries[0], "status=500")

	// 网络失败路径：warn 打点携带 error
	cl3 := newCaptureLogger()
	c3 := NewClient(WithLogger(cl3))
	_ = c3.DoJSON(context.Background(), MethodPost, "http://127.0.0.1:1", nil, nil, nil)
	entries = cl3.logs()
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0], "warn|[LLMX] http 网络失败")
	assert.Contains(t, entries[0], "error=")
}

func TestLogging_StreamRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentTypeEvent)
		_, _ = w.Write([]byte("data: {\"a\":1}\n\n"))
	}))
	defer srv.Close()

	cl := newCaptureLogger()
	c := NewClient(WithLogger(cl))
	require.NoError(t, c.DoStream(context.Background(), MethodPost, srv.URL, map[string]any{}, nil,
		func(ev SSEEvent) error { return nil }))
	entries := cl.logs()
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0], "debug|[LLMX] http 请求完成")
}

func TestSetLogger_NilIgnored(t *testing.T) {
	c := NewClient()
	prev := c.Logger
	c.SetLogger(nil)
	assert.Equal(t, prev, c.Logger) // nil 不生效防误清空

	// 正向：注入后生效
	next := newCaptureLogger()
	c.SetLogger(next)
	assert.Equal(t, logger.ILogger(next), c.Logger)
}
