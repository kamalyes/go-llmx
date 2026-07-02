/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 22:38:51
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 22:47:11
 * @FilePath: \go-llmx\adapters\cohere\stream.go
 * @Description: Cohere 流式增量聚合器 —— 按事件类型分发：
 * content-delta 透传文本，tool-call-delta 按 index 聚合参数拼接，
 * message-end 收口结束原因与用量
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccohere

import (
	"encoding/json"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// wireStreamEvent 单帧流式事件（覆盖全部事件类型的字段并集）.
// [EN] A single streaming event (field union over all event types).
type wireStreamEvent struct {
	// Type 事件类型（content-*/tool-call-*/message-end）.
	// [EN] Event type.
	Type string `json:"type"`

	// Index 内容序号（文本块与工具调用共用序号空间）.
	// [EN] Content index (shared by text and tool calls).
	Index int `json:"index"`

	// ID 调用 ID（tool-call-start 携带）.
	// [EN] Call ID (tool-call-start).
	ID string `json:"id,omitempty"`

	// Delta 增量载荷.
	// [EN] Delta payload.
	Delta *wireStreamDelta `json:"delta,omitempty"`
}

// wireStreamDelta 增量载荷.
// [EN] Delta payload.
type wireStreamDelta struct {
	// Message 消息增量.
	// [EN] Message delta.
	Message *wireStreamMessage `json:"message,omitempty"`

	// FinishReason 结束原因（message-end 携带）.
	// [EN] Finish reason (message-end).
	FinishReason string `json:"finish_reason,omitempty"`

	// Usage 用量（message-end 携带）.
	// [EN] Usage (message-end).
	Usage *wireUsage `json:"usage,omitempty"`
}

// wireStreamMessage 消息增量（文本 / 工具调用的字段并集）.
// [EN] Message delta (field union of text and tool calls).
type wireStreamMessage struct {
	// Content 文本增量.
	// [EN] Text delta.
	Content *wireStreamText `json:"content,omitempty"`

	// ToolCalls 工具调用增量.
	// [EN] Tool call delta.
	ToolCalls *wireStreamToolCall `json:"tool_calls,omitempty"`
}

// wireStreamText 文本增量.
// [EN] Text delta.
type wireStreamText struct {
	// Text 增量文本.
	// [EN] Delta text.
	Text string `json:"text,omitempty"`
}

// wireStreamToolCall 工具调用增量.
// [EN] Tool call delta.
type wireStreamToolCall struct {
	// Function 函数载荷增量（name / arguments 分片到达）.
	// [EN] Function payload delta.
	Function wireFunctionCall `json:"function,omitempty"`
}

// streamAggregator 流式聚合状态机.
// [EN] Streaming aggregation state machine.
type streamAggregator struct {
	// textBuf 文本累积.
	// [EN] Text accumulation.
	textBuf strings.Builder

	// finish 结束原因.
	// [EN] Finish reason.
	finish string

	// usage 用量.
	// [EN] Usage.
	usage llmx.Usage

	// toolCalls 工具调用按序号聚合.
	// [EN] Tool calls aggregated by index.
	toolCalls map[int]*llmx.ToolCallPart

	// order 工具调用出现顺序.
	// [EN] Tool call appearance order.
	order []int

	// sawContent 是否收到过任何内容信号（空流判定）.
	// [EN] Whether any content signal was seen.
	sawContent bool
}

// newStreamAggregator 构造聚合器.
// [EN] Build an aggregator.
func newStreamAggregator() *streamAggregator {
	return &streamAggregator{toolCalls: map[int]*llmx.ToolCallPart{}}
}

// feed 消费一帧 SSE data 并透传增量给回调.
// [EN] Consume one SSE data frame and forward deltas to the callback.
func (s *streamAggregator) feed(data string, stream llmx.StreamHandler) error {
	var ev wireStreamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		return nil // 非 JSON data 静默容忍
	}
	if ev.Delta == nil {
		return nil
	}

	switch ev.Type {
	case eventContentDelta:
		if ev.Delta.Message != nil && ev.Delta.Message.Content != nil {
			text := ev.Delta.Message.Content.Text
			if text == "" {
				return nil
			}
			s.sawContent = true
			s.textBuf.WriteString(text)
			if stream != nil {
				if err := stream(&llmx.Chunk{Content: text}); err != nil {
					return err
				}
			}
		}
	case eventToolCallStart, eventToolCallDelta:
		if ev.Delta.Message == nil || ev.Delta.Message.ToolCalls == nil {
			return nil
		}
		s.sawContent = true
		call, ok := s.toolCalls[ev.Index]
		if !ok {
			call = &llmx.ToolCallPart{}
			s.toolCalls[ev.Index] = call
			s.order = append(s.order, ev.Index)
		}
		if ev.ID != "" {
			call.ID = ev.ID
		}
		if name := ev.Delta.Message.ToolCalls.Function.Name; name != "" {
			call.Name = name
		}
		call.Arguments += ev.Delta.Message.ToolCalls.Function.Arguments
		if stream != nil {
			delta := &llmx.ToolCallDelta{Index: ev.Index, ID: ev.ID, Name: ev.Delta.Message.ToolCalls.Function.Name, Arguments: ev.Delta.Message.ToolCalls.Function.Arguments}
			if err := stream(&llmx.Chunk{ToolCallDelta: delta}); err != nil {
				return err
			}
		}
	case eventMessageEnd:
		s.sawContent = true
		if ev.Delta.FinishReason != "" {
			s.finish = mapFinishReason(ev.Delta.FinishReason)
		}
		if ev.Delta.Usage != nil {
			s.usage = llmx.Usage{
				PromptTokens:     ev.Delta.Usage.Tokens.InputTokens,
				CompletionTokens: ev.Delta.Usage.Tokens.OutputTokens,
				TotalTokens:      ev.Delta.Usage.Tokens.InputTokens + ev.Delta.Usage.Tokens.OutputTokens,
			}
		}
		if stream != nil {
			if err := stream(&llmx.Chunk{FinishReason: s.finish, Usage: s.usage}); err != nil {
				return err
			}
		}
	}
	return nil
}

// response 收口聚合结果.
// [EN] Finalize the aggregated response.
func (s *streamAggregator) response(model string) *llmx.Response {
	resp := &llmx.Response{Model: model, Usage: s.usage}
	if !s.sawContent {
		return resp
	}
	ch := llmx.Choice{FinishReason: s.finish}
	if s.textBuf.Len() > 0 {
		ch.Content = []llmx.Part{llmx.TextPart{Text: s.textBuf.String()}}
	}
	for _, idx := range s.order {
		if call := s.toolCalls[idx]; call != nil {
			if call.Arguments == "" {
				call.Arguments = emptyJSONObject
			}
			ch.Content = append(ch.Content, *call)
			if ch.FinishReason == "" || ch.FinishReason == "stop" {
				ch.FinishReason = "tool_calls"
			}
		}
	}
	resp.Choices = []llmx.Choice{ch}
	return resp
}
