/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 21:07:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 21:07:00
 * @FilePath: \go-llmx\adapters\anthropic\stream.go
 * @Description: Anthropic 流式增量聚合器 —— 按事件名分发 SSE 帧：
 * text_delta/thinking_delta 逐段透传，tool_use 块按 index 聚合参数拼接，
 * message_delta 收口结束原因与输出用量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcanthropic

import (
	"encoding/json"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/transport"
)

// wireStreamEvent 单帧流式事件（覆盖全部事件类型的字段并集）.
// [EN] A single streaming event (field union over all event types).
type wireStreamEvent struct {
	// Type 事件类型（与 event: 行一致，冗余校验用）.
	// [EN] Event type (mirrors the event: line).
	Type string `json:"type"`

	// Message 消息体（message_start 携带，含初始 usage）.
	// [EN] Message body (message_start, with initial usage).
	Message *wireResponse `json:"message,omitempty"`

	// Index 内容块序号（content_block_* 事件）.
	// [EN] Content block index (content_block_* events).
	Index int `json:"index"`

	// ContentBlock 内容块（content_block_start 携带，tool_use 含 id/name）.
	// [EN] Content block (content_block_start; tool_use carries id/name).
	ContentBlock *wireBlock `json:"content_block,omitempty"`

	// Delta 增量载荷（text_delta/input_json_delta/thinking_delta/结束原因）.
	// [EN] Delta payload (text/input_json/thinking/stop reason).
	Delta *wireStreamDeltaBody `json:"delta,omitempty"`

	// Usage 输出用量（message_delta 携带）.
	// [EN] Output usage (message_delta).
	Usage *wireUsage `json:"usage,omitempty"`

	// Error 流中错误（error 事件）.
	// [EN] Mid-stream error (error event).
	Error *wireErrorBody `json:"error,omitempty"`
}

// wireStreamDeltaBody 增量载荷.
// [EN] Delta payload.
type wireStreamDeltaBody struct {
	// Type 增量类型（text_delta/input_json_delta/thinking_delta）.
	// [EN] Delta type.
	Type string `json:"type"`

	// Text 文本增量.
	// [EN] Text delta.
	Text string `json:"text,omitempty"`

	// PartialJSON 工具参数 JSON 增量.
	// [EN] Tool arguments JSON delta.
	PartialJSON string `json:"partial_json,omitempty"`

	// Thinking 思考轨迹增量.
	// [EN] Thinking trace delta.
	Thinking string `json:"thinking,omitempty"`

	// StopReason 结束原因（message_delta）.
	// [EN] Stop reason (message_delta).
	StopReason string `json:"stop_reason,omitempty"`
}

// streamAggregator 流式聚合状态机.
// [EN] Streaming aggregation state machine.
type streamAggregator struct {
	// textBuf 文本累积.
	// [EN] Text accumulation.
	textBuf strings.Builder

	// reasonBuf 思考轨迹累积.
	// [EN] Thinking trace accumulation.
	reasonBuf strings.Builder

	// finish 结束原因.
	// [EN] Finish reason.
	finish string

	// inputTokens / outputTokens 用量.
	// [EN] Usage counters.
	inputTokens  int
	outputTokens int

	// toolBlocks 工具调用按块序号聚合（Arguments 逐段拼接）.
	// [EN] Tool calls aggregated by block index.
	toolBlocks map[int]*llmx.ToolCallPart

	// maxToolIndex 已见过的最大块序号（块序号为协议序号，非 0 起连续）.
	// [EN] Max block index seen (protocol indices are not contiguous from 0).
	maxToolIndex int

	// sawContent 是否收到过任何内容信号（空流判定）.
	// [EN] Whether any content signal was seen.
	sawContent bool
}

// newStreamAggregator 构造聚合器.
// [EN] Build an aggregator.
func newStreamAggregator() *streamAggregator {
	return &streamAggregator{toolBlocks: map[int]*llmx.ToolCallPart{}, maxToolIndex: -1}
}

// feed 消费一帧 SSE 事件并透传增量给回调.
// [EN] Consume one SSE event and forward deltas to the callback.
//
// 按 event: 行分发；非 JSON data 静默容忍（网关注释/心跳以 data 形态混入）
func (s *streamAggregator) feed(ev transport.SSEEvent, stream llmx.StreamHandler) error {
	var e wireStreamEvent
	if err := json.Unmarshal([]byte(ev.Data), &e); err != nil {
		return nil
	}

	switch name := eventOf(ev, e); name {
	case eventMessageStart:
		if e.Message != nil {
			s.inputTokens = e.Message.Usage.InputTokens
		}

	case eventContentBlockStart:
		s.sawContent = true
		if e.ContentBlock != nil && e.ContentBlock.Type == blockTypeToolUse {
			s.toolBlocks[e.Index] = &llmx.ToolCallPart{ID: e.ContentBlock.ID, Name: e.ContentBlock.Name}
			s.maxToolIndex = max(s.maxToolIndex, e.Index)
			// 工具块开始帧透传回调（携带 ID/Name，Arguments 由后续 delta 帧补齐）
			if err := stream(&llmx.Chunk{ToolCallDelta: &llmx.ToolCallDelta{
				Index: e.Index,
				ID:    e.ContentBlock.ID,
				Name:  e.ContentBlock.Name,
			}}); err != nil {
				return err
			}
		}

	case eventContentBlockDelta:
		s.sawContent = true
		if e.Delta == nil {
			return nil
		}
		switch e.Delta.Type {
		case deltaTypeText:
			s.textBuf.WriteString(e.Delta.Text)
			if err := stream(&llmx.Chunk{Content: e.Delta.Text}); err != nil {
				return err
			}
		case deltaTypeThinking:
			s.reasonBuf.WriteString(e.Delta.Thinking)
			if err := stream(&llmx.Chunk{Reasoning: e.Delta.Thinking}); err != nil {
				return err
			}
		case deltaTypeInputJSON:
			agg, ok := s.toolBlocks[e.Index]
			if !ok {
				// 兜底：未见过 content_block_start 的乱序帧
				agg = &llmx.ToolCallPart{}
				s.toolBlocks[e.Index] = agg
				s.maxToolIndex = max(s.maxToolIndex, e.Index)
			}
			agg.Arguments += e.Delta.PartialJSON
			// 参数增量帧透传回调（Arguments 为本帧片段，非全量）
			if err := stream(&llmx.Chunk{ToolCallDelta: &llmx.ToolCallDelta{
				Index:     e.Index,
				Arguments: e.Delta.PartialJSON,
			}}); err != nil {
				return err
			}
		}

	case eventMessageDelta:
		if e.Usage != nil {
			s.outputTokens = e.Usage.OutputTokens
		}
		if e.Delta != nil && e.Delta.StopReason != "" {
			s.finish = e.Delta.StopReason
			// 结束信号帧：携带结束原因与最终用量
			return stream(&llmx.Chunk{FinishReason: s.finish, Usage: s.usage()})
		}

	case eventMessageStop:
		return errStopReading

	case eventPing:
		// 心跳，忽略

	case errorTypeAnthropic:
		if e.Error != nil {
			return &streamError{body: e.Error}
		}
	}
	return nil
}

// eventOf 事件名解析（event: 行优先，data 内 type 兜底）.
// [EN] Resolve the event name (event: line first, data type as fallback).
func eventOf(ev transport.SSEEvent, e wireStreamEvent) string {
	if ev.Event != "" {
		return ev.Event
	}
	return e.Type
}

// usage 输出聚合后的用量.
// [EN] Emit the aggregated usage.
func (s *streamAggregator) usage() llmx.Usage {
	return llmx.Usage{
		PromptTokens:     s.inputTokens,
		CompletionTokens: s.outputTokens,
		TotalTokens:      s.inputTokens + s.outputTokens,
	}
}

// response 输出聚合后的完整响应.
// [EN] Emit the aggregated full response.
func (s *streamAggregator) response(model string) *llmx.Response {
	choice := llmx.Choice{
		FinishReason: s.finish,
		Reasoning:    s.reasonBuf.String(),
	}
	if s.textBuf.Len() > 0 {
		choice.Content = append(choice.Content, llmx.TextPart{Text: s.textBuf.String()})
	}
	// 按块序号升序输出聚合后的工具调用（与到达顺序一致；序号为协议序号非 0 起连续）
	for i := 0; i <= s.maxToolIndex; i++ {
		if tc, ok := s.toolBlocks[i]; ok {
			choice.Content = append(choice.Content, *tc)
		}
	}

	resp := &llmx.Response{Usage: s.usage(), Model: model}
	if s.sawContent || len(choice.Content) > 0 || choice.Reasoning != "" {
		resp.Choices = []llmx.Choice{choice}
	}
	return resp
}

// streamError 流中 error 事件（由 mapTransportError 统一映射）.
// [EN] Mid-stream error event (mapped by mapTransportError).
type streamError struct {
	body *wireErrorBody
}

// Error 实现 error.
// [EN] Implement error.
func (e *streamError) Error() string {
	if e.body == nil {
		return errorTypeAnthropic
	}
	return e.body.Type + ": " + e.body.Message
}
