/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 21:53:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 21:53:00
 * @FilePath: \go-llmx\adapters\anthropic\stream_test.go
 * @Description: Anthropic 适配器流式聚合测试 —— 按事件名分发的增量聚合：
 * text/thinking/tool 参数拼接、ping/注释/非 JSON 帧容错、中断与空流路径.
 * mock 基建见 anthropic_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcanthropic

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
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: message_start\n")
		fmt.Fprint(w, `data: {"type":"message_start","message":{"usage":{"input_tokens":8,"output_tokens":0}}}`+"\n\n")
		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你"}}`+"\n\n")
		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"好"}}`+"\n\n")
		fmt.Fprint(w, "event: message_delta\n")
		fmt.Fprint(w, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":6}}`+"\n\n")
		fmt.Fprint(w, "event: message_stop\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	var text string
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(ch *llmx.Chunk) error {
		if ch.Content != "" {
			text += ch.Content
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "你好", text)
	assert.Equal(t, "你好", resp.Choices[0].Text())
	assert.Equal(t, "end_turn", resp.Choices[0].FinishReason)
	assert.Equal(t, 8, resp.Usage.PromptTokens)
	assert.Equal(t, 6, resp.Usage.CompletionTokens)
	assert.Equal(t, 14, resp.Usage.TotalTokens)
}

func TestStreamGenerateContent_ThinkingAndToolUse(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"想一想"}}`+"\n\n")
		fmt.Fprint(w, "event: content_block_start\n")
		fmt.Fprint(w, `data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_1","name":"calc"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"a\":"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"1}"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	var reasoning string
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(ch *llmx.Chunk) error {
		if ch.Reasoning != "" {
			reasoning += ch.Reasoning
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "想一想", reasoning)
	assert.Equal(t, "想一想", resp.Choices[0].Reasoning)

	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "tu_1", calls[0].ID)
	assert.Equal(t, "calc", calls[0].Name)
	// 参数 JSON 分段拼接
	assert.JSONEq(t, `{"a":1}`, calls[0].Arguments)
}

func TestStreamToolCallDelta_Forwarded(t *testing.T) {
	// 工具块增量透传回调：block_start 帧携带 ID/Name，参数 delta 逐段
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: content_block_start\n")
		fmt.Fprint(w, `data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"tu_9","name":"weather"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"北京\"}"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	var deltas []*llmx.ToolCallDelta
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(ch *llmx.Chunk) error {
		if ch.ToolCallDelta != nil {
			deltas = append(deltas, ch.ToolCallDelta)
		}
		return nil
	})
	require.NoError(t, err)

	// 透传保真：首帧 block_start 带 ID/Name（Arguments 空），参数帧带片段
	require.Len(t, deltas, 3)
	assert.Equal(t, 2, deltas[0].Index)
	assert.Equal(t, "tu_9", deltas[0].ID)
	assert.Equal(t, "weather", deltas[0].Name)
	assert.Empty(t, deltas[0].Arguments)
	assert.Equal(t, `{"city":`, deltas[1].Arguments)
	assert.Equal(t, `"北京"}`, deltas[2].Arguments)

	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 1)
	assert.JSONEq(t, `{"city":"北京"}`, calls[0].Arguments)
}

func TestStreamGenerateContent_ToleranceAndStop(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": heartbeat\n\n")    // 注释行
		fmt.Fprint(w, "event: ping\n")     // ping 事件
		fmt.Fprint(w, `data: {"type":"ping"}`+"\n\n")
		fmt.Fprint(w, "data: not-json\n\n") // 非 JSON 帧
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "hi", resp.Choices[0].Text())
}

func TestStreamGenerateContent_StopEarly(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"部分"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
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

func TestStreamGenerateContent_EmptyFlow(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
}

func TestStreamGenerateContent_NilHandler(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "直通"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	// stream 为 nil → 退化为非流式
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, nil)
	require.NoError(t, err)
	assert.Equal(t, "直通", resp.Choices[0].Text())
}

func TestStreamFeed_EdgeBranches(t *testing.T) {
	// 载荷缺失/空字段的事件帧全部容忍：message_start 无 message、
	// delta 载荷为空、content_block_stop 事件、message_delta 仅携带 usage、
	// error 事件无 error 载荷
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"message_start"}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":0}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"content_block_stop","index":0}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_delta","usage":{"output_tokens":3}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"error"}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	require.NoError(t, err)
	// message_start 无 usage 载荷 → 输入 0；message_delta usage 生效
	assert.Equal(t, 3, resp.Usage.CompletionTokens)
}

func TestStreamMessageStart_CarriesUsage(t *testing.T) {
	// message_start 初始 usage 生效（无 message_delta usage 覆盖输入侧）
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":0}}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	// 空内容 → ErrEmptyResponse（usage 已收但无内容块）
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
	assert.Equal(t, 5, resp.Usage.PromptTokens)
}

func TestStreamThinkingChunkError(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"想"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	// handler 在 thinking 增量帧返回错误 → 透传中断
	wantErr := errors.New("thinking aborted")
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(ch *llmx.Chunk) error {
		if ch.Reasoning != "" {
			return wantErr
		}
		return nil
	})
	assert.ErrorIs(t, err, wantErr)
}

func TestStreamToolDeltaWithoutBlockStart(t *testing.T) {
	// 乱序容错：input_json_delta 先于 content_block_start 到达 → 兜底建块聚合
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"x\":"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"9}"}}`+"\n\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	require.NoError(t, err)
	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 1)
	assert.JSONEq(t, `{"x":9}`, calls[0].Arguments)
}
