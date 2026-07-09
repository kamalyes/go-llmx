/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-25 21:16:58
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:38:16
 * @FilePath: \go-llmx\embeddings\ollama\logging_test.go
 * @Description: ctx + logger 端到端测试 —— embed 打点（批量 count=N / 检索 count=1 的
 * input 两形态收口）、业务 ctx 追踪值贯通、取消语义跨层保留. mock 基建见 embedder_test.go
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcollamaembed

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

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

// e2eLogger 端到端日志捕获器（嵌入空实现，覆写 Debug/Info/Warn 三方法）.
// [EN] E2E log capturer (overrides Debug/Info/Warn ContextKV methods).
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

func (l *e2eLogger) DebugContextKV(ctx context.Context, msg string, kv ...interface{}) {
	l.record(ctx, "debug", msg, kv...)
}
func (l *e2eLogger) InfoContextKV(ctx context.Context, msg string, kv ...interface{}) {
	l.record(ctx, "info", msg, kv...)
}
func (l *e2eLogger) WarnContextKV(ctx context.Context, msg string, kv ...interface{}) {
	l.record(ctx, "warn", msg, kv...)
}

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

func TestE2E_EmbedLogging_InputShapes(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"embeddings":[[0.1,0.2],[0.3,0.4]]}`)
	})

	cl := newE2ELogger()
	c := New("k", WithBaseURL(m.srv.URL), WithModel("nomic-embed-text"), WithLogger(cl))
	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-ollama-embed")

	// 索引阶段：[]string 批量 → count=2
	vectors, err := c.EmbedDocuments(ctx, []string{"文档一", "文档二"})
	require.NoError(t, err)
	require.Len(t, vectors, 2)

	entries := cl.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, "info|[LLMX] embed", entries[0].level+"|"+entries[0].msg)
	v, ok := kvVal(entries[0].kv, "count")
	require.True(t, ok)
	assert.Equal(t, 2, v, "批量嵌入 count 应为文档数")
	assert.Equal(t, "debug|[LLMX] http 请求完成", entries[1].level+"|"+entries[1].msg)

	// 检索阶段：string 单条形态 → count=1（协议 input 两形态收口的打点分支）
	cl2 := newE2ELogger()
	c2 := New("k", WithBaseURL(m.srv.URL), WithLogger(cl2))
	_, err = c2.EmbedQuery(ctx, "查询词")
	require.NoError(t, err)

	entries = cl2.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, "[LLMX] embed", entries[0].msg)
	v, ok = kvVal(entries[0].kv, "count")
	require.True(t, ok)
	assert.Equal(t, 1, v, "检索嵌入（string 形态）count 应为 1")

	// 业务 ctx 贯通到每条打点
	for _, e := range entries {
		assert.Equal(t, "trace-ollama-embed", e.ctx.Value(ctxKey{}))
	}
}

func TestE2E_ContextCancelled(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("取消的 ctx 不应发出请求")
	})

	cl := newE2ELogger()
	c := New("k", WithBaseURL(m.srv.URL), WithLogger(cl))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.EmbedQuery(ctx, "查询词")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled, "ctx 取消语义应跨层保留")

	entries := cl.snapshot()
	require.Len(t, entries, 2)
	assert.Equal(t, "[LLMX] embed", entries[0].msg)
	assert.Equal(t, "warn|[LLMX] http 网络失败", entries[1].level+"|"+entries[1].msg)
}
