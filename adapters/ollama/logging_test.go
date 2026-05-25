/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-25 21:02:26
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-25 21:02:26
 * @FilePath: \go-llmx\adapters\ollama\logging_test.go
 * @Description: ctx + logger 端到端测试 —— NDJSON 流式打点双维度、业务 ctx 追踪值贯通、
 * 取消语义跨层保留. mock 基建见 ollama_test.go
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcollama

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ctxKey E2E 链路追踪键.
// [EN] E2E tracing key.
type ctxKey struct{}

// logEntry 单条捕获（含 ctx）.
// [EN] One captured entry (with ctx).
type logEntry struct {
	ctx   context.Context
	level string
	msg   string
	kv    []interface{}
}

// e2eLogger 端到端日志捕获器（嵌入空实现，仅覆写 ContextKV 四方法）.
// [EN] E2E log capturer (overrides the four ContextKV methods).
type e2eLogger struct {
	*logger.EmptyLogger
	mu      sync.Mutex
	entries []logEntry
}

func newE2ELogger() *e2eLogger {
	return &e2eLogger{EmptyLogger: logger.NewEmptyLogger()}
}

func (l *e2eLogger) record(ctx context.Context, level, msg string, kv ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, logEntry{ctx: ctx, level: level, msg: msg, kv: append([]interface{}(nil), kv...)})
}

func (l *e2eLogger) DebugContextKV(ctx context.Context, msg string, kv ...interface{}) { l.record(ctx, "debug", msg, kv...) }
func (l *e2eLogger) InfoContextKV(ctx context.Context, msg string, kv ...interface{})  { l.record(ctx, "info", msg, kv...) }
func (l *e2eLogger) WarnContextKV(ctx context.Context, msg string, kv ...interface{})  { l.record(ctx, "warn", msg, kv...) }
func (l *e2eLogger) ErrorContextKV(ctx context.Context, msg string, kv ...interface{}) { l.record(ctx, "error", msg, kv...) }

// snapshot 读取捕获条目快照.
// [EN] Snapshot the captured entries.
func (l *e2eLogger) snapshot() []logEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]logEntry(nil), l.entries...)
}

// assertTraceFlows 断言每条打点的 ctx 均携带业务注入的追踪值.
// [EN] Assert every entry's ctx carries the injected trace value.
func assertTraceFlows(t *testing.T, entries []logEntry, want string) {
	t.Helper()
	require.NotEmpty(t, entries)
	for _, e := range entries {
		v, ok := e.ctx.Value(ctxKey{}).(string)
		require.True(t, ok, "打点 %q 的 ctx 未携带业务追踪值", e.msg)
		assert.Equal(t, want, v)
	}
}

func TestE2E_LoggingAndContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-ollama")

	// 非流式：[LLMX] chat + http 请求完成
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"ok"},"done":true,"done_reason":"stop","prompt_eval_count":1,"eval_count":1}`)
	})
	cl := newE2ELogger()
	c := New("k", WithBaseURL(m.srv.URL), WithModel("llama3.2"), WithLogger(cl))
	_, err := c.GenerateContent(ctx, []llmx.Message{llmx.User("q")})
	require.NoError(t, err)

	entries := cl.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, "info|[LLMX] chat", entries[0].level+"|"+entries[0].msg)
	assert.Equal(t, "debug|[LLMX] http 请求完成", entries[1].level+"|"+entries[1].msg)
	assertTraceFlows(t, entries, "trace-ollama")

	// 流式（NDJSON）：[LLMX] stream + http 请求完成
	ms := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"ok"},"done":true,"done_reason":"stop"}`+"\n")
	})
	cl2 := newE2ELogger()
	c2 := New("k", WithBaseURL(ms.srv.URL), WithLogger(cl2))
	_, err = c2.StreamGenerateContent(ctx, []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error { return nil })
	require.NoError(t, err)

	entries = cl2.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, "info|[LLMX] stream", entries[0].level+"|"+entries[0].msg)
	assert.Equal(t, "debug|[LLMX] http 请求完成", entries[1].level+"|"+entries[1].msg)
	assertTraceFlows(t, entries, "trace-ollama")
}

func TestE2E_ContextCancelled(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("取消的 ctx 不应发出请求")
	})

	cl := newE2ELogger()
	c := New("k", WithBaseURL(m.srv.URL), WithLogger(cl))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GenerateContent(ctx, []llmx.Message{llmx.User("q")})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled, "ctx 取消语义应跨层保留")
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)

	entries := cl.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, "[LLMX] chat", entries[0].msg)
	assert.Equal(t, "warn|[LLMX] http 网络失败", entries[1].level+"|"+entries[1].msg)
}
