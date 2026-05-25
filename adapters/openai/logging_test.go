/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-25 20:26:52
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-25 20:26:52
 * @FilePath: \go-llmx\adapters\openai\logging_test.go
 * @Description: ctx + logger 端到端测试 —— 业务调用 → 适配器打点 → 传输打点 → HTTP 收发全链路.
 * 验证三件事：日志双维度（[LLMX] chat/stream + http 请求完成）、业务 ctx 携带的追踪值
 * 贯通到每条打点、ctx 取消语义跨层保留（errors.Is 可判定）. mock 基建见 openai_test.go
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcopenai

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

// ctxKey E2E 链路追踪键（模拟业务侧 trace 注入）.
// [EN] E2E tracing key (mimics business-side trace injection).
type ctxKey struct{}

// logEntry 单条捕获（含 ctx，验证全链路传递的是业务 ctx 而非 Background）.
// [EN] One captured entry (with ctx, proving the business ctx flows through).
type logEntry struct {
	ctx   context.Context
	level string
	msg   string
	kv    []interface{}
}

// e2eLogger 端到端日志捕获器（嵌入空实现，仅覆写 ContextKV 四方法）.
// [EN] E2E log capturer (embeds the empty impl, overriding the four ContextKV methods).
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

// kvVal 取键值对中指定键的值.
// [EN] Look up a value by key in the kv pairs.
func kvVal(kv []interface{}, key string) (interface{}, bool) {
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i] == key {
			return kv[i+1], true
		}
	}
	return nil, false
}

// assertTraceFlows 断言每条打点的 ctx 均携带业务注入的追踪值.
// [EN] Assert every captured entry's ctx carries the injected trace value.
func assertTraceFlows(t *testing.T, entries []logEntry, want string) {
	t.Helper()
	require.NotEmpty(t, entries)
	for _, e := range entries {
		v, ok := e.ctx.Value(ctxKey{}).(string)
		require.True(t, ok, "打点 %q 的 ctx 未携带业务追踪值", e.msg)
		assert.Equal(t, want, v, "打点 %q 的 ctx 追踪值不符", e.msg)
	}
}

func TestE2E_ChatLogging_FullChain(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	})

	cl := newE2ELogger()
	c := New("k", WithBaseURL(m.srv.URL), WithModel("gpt-4o-mini"), WithLogger(cl))
	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-e2e-chat")

	_, err := c.GenerateContent(ctx, []llmx.Message{llmx.User("hi"), llmx.User("again")})
	require.NoError(t, err)

	entries := cl.snapshot()
	require.Len(t, entries, 2, "应恰好两条打点：模型维度 + 传输维度")

	// 第一条：模型维度（info，model/count 业务语义）
	assert.Equal(t, "info", entries[0].level)
	assert.Equal(t, "[LLMX] chat", entries[0].msg)
	v, ok := kvVal(entries[0].kv, "model")
	require.True(t, ok)
	assert.Equal(t, "gpt-4o-mini", v)
	v, ok = kvVal(entries[0].kv, "count")
	require.True(t, ok)
	assert.Equal(t, 2, v)

	// 第二条：传输维度（debug，method/url/status/elapsed HTTP 语义）
	assert.Equal(t, "debug", entries[1].level)
	assert.Equal(t, "[LLMX] http 请求完成", entries[1].msg)
	v, ok = kvVal(entries[1].kv, "method")
	require.True(t, ok)
	assert.Equal(t, "POST", v)
	v, ok = kvVal(entries[1].kv, "status")
	require.True(t, ok)
	assert.Equal(t, 200, v)
	_, ok = kvVal(entries[1].kv, "elapsed")
	assert.True(t, ok, "传输打点应携带 elapsed")

	// 业务 ctx 贯通到每条打点（go-logger extractContextInfo 同源消费）
	assertTraceFlows(t, entries, "trace-e2e-chat")
}

func TestE2E_StreamLogging_FullChain(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"你"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"好"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})

	cl := newE2ELogger()
	c := New("k", WithBaseURL(m.srv.URL), WithLogger(cl))
	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-e2e-stream")

	_, err := c.StreamGenerateContent(ctx, []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error { return nil })
	require.NoError(t, err)

	entries := cl.snapshot()
	require.Len(t, entries, 2)

	assert.Equal(t, "info", entries[0].level)
	assert.Equal(t, "[LLMX] stream", entries[0].msg)
	assert.Equal(t, "debug", entries[1].level)
	assert.Equal(t, "[LLMX] http 请求完成", entries[1].msg)
	assertTraceFlows(t, entries, "trace-e2e-stream")
}

func TestE2E_ContextCancelled_SemanticsPreserved(t *testing.T) {
	// 预取消 ctx：http.Client 发出请求前即失败（服务端不会被调用）
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("取消的 ctx 不应发出请求")
	})

	cl := newE2ELogger()
	c := New("k", WithBaseURL(m.srv.URL), WithLogger(cl))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 非流式：取消语义 + provider 哨兵双层可判定
	_, err := c.GenerateContent(ctx, []llmx.Message{llmx.User("q")})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled, "ctx 取消语义应跨层保留")
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)

	// 打点：chat 打点 + 网络失败打点（取消发生在请求前，chat 打点先行）
	entries := cl.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, "[LLMX] chat", entries[0].msg)
	assert.Equal(t, "warn", entries[1].level)
	assert.Equal(t, "[LLMX] http 网络失败", entries[1].msg)

	// 流式路径同样保留取消语义
	cl2 := newE2ELogger()
	c2 := New("k", WithBaseURL(m.srv.URL), WithLogger(cl2))
	_, err = c2.StreamGenerateContent(ctx, []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error { return nil })
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	entries = cl2.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, "[LLMX] stream", entries[0].msg)
}
