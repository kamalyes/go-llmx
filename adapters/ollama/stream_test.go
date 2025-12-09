/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 21:37:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 21:37:00
 * @FilePath: \go-llmx\adapters\ollama\stream_test.go
 * @Description: Ollama 适配器流式聚合测试 —— NDJSON 逐行增量聚合：
 * 文本透传、工具调用帧聚合、done 帧收口结束原因与用量、
 * 中断/空流/重复工具帧/缺省 done_reason 路径.
 * mock 基建见 ollama_test.go
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamGenerateContent_FullFlow(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"你"},"done":false}`+"\n")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"好"},"done":false}`+"\n")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":8,"eval_count":6}`+"\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	var text string
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, llmx.StreamCollector(&text))
	require.NoError(t, err)

	assert.Equal(t, "你好", text)
	assert.Equal(t, "你好", resp.Choices[0].Text())
	assert.Equal(t, "stop", resp.Choices[0].FinishReason)
	// done 帧收口用量（无 total 字段，本地求和）
	assert.Equal(t, 8, resp.Usage.PromptTokens)
	assert.Equal(t, 6, resp.Usage.CompletionTokens)
	assert.Equal(t, 14, resp.Usage.TotalTokens)
	// 流式请求体断言
	assert.Equal(t, true, m.body("stream"))
}

func TestStreamGenerateContent_ToolUse(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		// 协议：工具调用在 done 帧一次性给全（非增量拼接）
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"查询中","tool_calls":[{"function":{"name":"calc","arguments":{"a":1}}},{"function":{"name":"search","arguments":{"q":"x"}}}]},"done":true,"done_reason":"stop"}`+"\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	var toolDeltas int
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(ch *llmx.Chunk) error {
		if ch.ToolCallDelta != nil {
			toolDeltas++
		}
		return nil
	})
	require.NoError(t, err)

	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, "calc", calls[0].Name)
	assert.JSONEq(t, `{"a":1}`, calls[0].Arguments)
	assert.Equal(t, "search", calls[1].Name)
	assert.JSONEq(t, `{"q":"x"}`, calls[1].Arguments)
	// 每个工具调用透传一个 ToolCallDelta
	assert.Equal(t, 2, toolDeltas)
}

func TestStreamGenerateContent_Tolerance(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, "\n")               // 空行
		fmt.Fprint(w, "not-json\n")        // 非 JSON 行容忍
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"hi"},"done":false}`+"\n")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":""},"done":true}`+"\n") // 缺省 done_reason
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "hi", resp.Choices[0].Text())
	// 缺省 done_reason → stop 兜底
	assert.Equal(t, "stop", resp.Choices[0].FinishReason)
}

func TestStreamGenerateContent_LastLineWithoutNewline(t *testing.T) {
	// transport 边界：末行无尾随换行符仍作为最后一帧收口
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"尾行"},"done":true,"done_reason":"stop"}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "尾行", resp.Choices[0].Text())
}

func TestStreamGenerateContent_StopEarly(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"部分"},"done":false}`+"\n")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`+"\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return llmx.ErrStopStream
	})
	// handler 主动终止 → 流关闭哨兵（正常语义）
	assert.ErrorIs(t, err, llmx.ErrStreamClosed)
	// 中断前仍返回已聚合内容
	assert.Equal(t, "部分", resp.Choices[0].Text())
}

func TestStreamGenerateContent_HandlerError(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"x"},"done":false}`+"\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	wantErr := errors.New("aborted")
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return wantErr
	})
	assert.ErrorIs(t, err, wantErr)
}

func TestStreamGenerateContent_EmptyFlow(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, "not-json\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
}

func TestStreamToolCallDelta_HandlerError(t *testing.T) {
	// 工具调用增量帧回调错误 → 透传中断
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, `{"message":{"role":"assistant","tool_calls":[{"function":{"name":"calc","arguments":{"a":1}}}]},"done":true,"done_reason":"stop"}`+"\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	wantErr := errors.New("tool delta aborted")
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(ch *llmx.Chunk) error {
		if ch.ToolCallDelta != nil {
			return wantErr
		}
		return nil
	})
	assert.ErrorIs(t, err, wantErr)
}

func TestStreamToolCall_RepeatFrameDedup(t *testing.T) {
	// 兜底路径：done 帧与前帧重复携带同名工具调用 → 去重保留首帧
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, `{"message":{"role":"assistant","tool_calls":[{"function":{"name":"calc","arguments":{"a":1}}}]},"done":false}`+"\n")
		fmt.Fprint(w, `{"message":{"role":"assistant","tool_calls":[{"function":{"name":"calc","arguments":{"a":1}}}]},"done":true,"done_reason":"stop"}`+"\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	require.NoError(t, err)
	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "call_calc", calls[0].ID)
}
