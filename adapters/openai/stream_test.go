/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-01 21:38:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-01 21:38:00
 * @FilePath: \go-llmx\adapters\openai\stream_test.go
 * @Description: OpenAI 适配器流式聚合测试 —— 文本/思考/工具调用增量聚合、容错
 * （注释行/非 JSON 帧/usage-only 帧）与中断路径. mock 基建见 openai_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcopenai

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

func TestStreamToolCalls(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"weather","arguments":"{\"ci"}}]}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ty\":\"北京\"}"}}]}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("北京天气")}, func(*llmx.Chunk) error { return nil })
	require.NoError(t, err)

	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "call_1", calls[0].ID)
	assert.Equal(t, "weather", calls[0].Name)
	assert.Equal(t, `{"city":"北京"}`, calls[0].Arguments)
	assert.Equal(t, "tool_calls", resp.Choices[0].FinishReason)
}

func TestStreamToolCallDelta_Forwarded(t *testing.T) {
	// 工具调用增量逐帧透传回调：首帧携带 ID/Name，参数按帧分段
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_x","function":{"name":"search","arguments":"{\"q\":"}}]}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"\"go\"}"}}]}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_0","function":{"name":"calc","arguments":"{}"}}]}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
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

	// 透传保真：帧序/首帧 ID+Name/参数分段逐帧到达
	require.Len(t, deltas, 3)
	assert.Equal(t, 1, deltas[0].Index)
	assert.Equal(t, "call_x", deltas[0].ID)
	assert.Equal(t, "search", deltas[0].Name)
	assert.Equal(t, `{"q":`, deltas[0].Arguments) // 本帧片段，非全量
	assert.Equal(t, `"go"}`, deltas[1].Arguments)
	assert.Equal(t, "call_0", deltas[2].ID)

	// 聚合侧：乱序到达（1 先于 0）仍按 index 升序输出完整调用
	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, "calc", calls[0].Name)
	assert.Equal(t, "search", calls[1].Name)
	assert.Equal(t, `{"q":"go"}`, calls[1].Arguments)
}

func TestStreamStopEarly(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"a"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"b"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"c"}}]}`+"\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	var n int
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		n++
		if n >= 2 {
			return llmx.ErrStopStream
		}
		return nil
	})
	assert.ErrorIs(t, err, llmx.ErrStreamClosed)
	assert.Equal(t, 2, n)
}

func TestStreamNilHandlerFallback(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "一次性"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, nil)
	require.NoError(t, err)
	assert.Equal(t, "一次性", resp.Choices[0].Text())
	// stream=false 走非流式路径
	assert.Nil(t, m.body("stream"))
}

func TestStreamReasoningAndTolerance(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": heartbeat\n\n")    // 注释行容忍
		fmt.Fprint(w, "data: not-json\n\n") // 非 JSON 帧容忍
		fmt.Fprint(w, `data: {"choices":[{"delta":{"reasoning_content":"想一想"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"答"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	var reasoning, text string
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(ch *llmx.Chunk) error {
		if ch.Reasoning != "" {
			reasoning += ch.Reasoning
		}
		if ch.Content != "" {
			text += ch.Content
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "想一想", reasoning)
	assert.Equal(t, "答", text)
	assert.Equal(t, "想一想", resp.Choices[0].Reasoning)
}

func TestStreamEmptyFlow(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n") // 无任何 choice 帧
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
}

func TestStreamUsageOnlyFrame(t *testing.T) {
	// stream_options include_usage 场景：末尾 choices 为空数组、仅携带 usage
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"hi"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":8,"total_tokens":15}}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(*llmx.Chunk) error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "hi", resp.Choices[0].Text())
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestStreamFinishChunkError(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	// handler 在结束帧（FinishReason chunk）返回错误 → 透传中断
	wantErr := errors.New("finish aborted")
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(ch *llmx.Chunk) error {
		if ch.FinishReason != "" {
			return wantErr
		}
		return nil
	})
	assert.ErrorIs(t, err, wantErr)
}

func TestStreamReasoningChunkError(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"reasoning_content":"想"}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})

	c := New("k", WithBaseURL(m.srv.URL))
	// handler 在 reasoning 增量帧返回错误 → 透传中断
	wantErr := errors.New("reasoning aborted")
	_, err := c.StreamGenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, func(ch *llmx.Chunk) error {
		if ch.Reasoning != "" {
			return wantErr
		}
		return nil
	})
	assert.ErrorIs(t, err, wantErr)
}
