/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 21:09:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 21:09:00
 * @FilePath: \go-llmx\adapters\ollama\stream.go
 * @Description: Ollama 流式增量聚合器 —— 消费 NDJSON 单帧：
 * content 逐段透传，done 帧收口结束原因与用量、工具调用按函数名去重聚合
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollama

import (
	"encoding/json"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// streamAggregator 流式聚合状态机.
// [EN] Streaming aggregation state machine.
type streamAggregator struct {
	// textBuf 文本累积.
	// [EN] Text accumulation.
	textBuf strings.Builder

	// finish 结束原因（done 帧收口）.
	// [EN] Finish reason (from the done frame).
	finish string

	// usage token 用量（done 帧收口）.
	// [EN] Token usage (from the done frame).
	usage llmx.Usage

	// toolCalls 工具调用聚合（协议一次给全，按函数名去重兜底重复帧）.
	// [EN] Aggregated tool calls (deduplicated by name for repeated frames).
	toolCalls []llmx.ToolCallPart

	// sawFrame 是否收到过任何有效帧（空流判定）.
	// [EN] Whether any valid frame was seen.
	sawFrame bool
}

// newStreamAggregator 构造聚合器.
// [EN] Build an aggregator.
func newStreamAggregator() *streamAggregator {
	return &streamAggregator{}
}

// feed 消费一行 NDJSON 并透传增量给回调.
// [EN] Consume one NDJSON line and forward deltas to the callback.
//
// 非 JSON 行静默容忍（网关注释/心跳以行形态混入）；done 帧补发收口 Chunk
func (s *streamAggregator) feed(line string, stream llmx.StreamHandler) error {
	var wr wireResponse
	if err := json.Unmarshal([]byte(line), &wr); err != nil {
		return nil
	}
	if wr.Error != "" {
		return &streamError{message: wr.Error}
	}

	s.sawFrame = true
	if wr.Message.Content != "" {
		s.textBuf.WriteString(wr.Message.Content)
		if err := stream(&llmx.Chunk{Content: wr.Message.Content}); err != nil {
			return err
		}
	}
	for _, tc := range wr.Message.ToolCalls {
		call := llmx.ToolCallPart{
			ID:        toolCallIDPrefix + tc.Function.Name,
			Name:      tc.Function.Name,
			Arguments: argumentsToJSON(tc.Function.Arguments),
		}
		if !s.appendToolCall(call) {
			continue
		}
		if err := stream(&llmx.Chunk{ToolCallDelta: &llmx.ToolCallDelta{
			Index:     len(s.toolCalls) - 1,
			ID:        call.ID,
			Name:      call.Name,
			Arguments: call.Arguments,
		}}); err != nil {
			return err
		}
	}

	// done 帧收口：结束原因 + 用量
	if wr.Done {
		s.finish = orDefaultFinish(wr.DoneReason)
		s.usage = usage(&wr)
		return stream(&llmx.Chunk{FinishReason: s.finish, Usage: s.usage})
	}
	return nil
}

// appendToolCall 追加工具调用（同名重复帧忽略，保持首帧）.
// [EN] Append a tool call (repeated names ignored, first frame kept).
func (s *streamAggregator) appendToolCall(call llmx.ToolCallPart) bool {
	for _, existing := range s.toolCalls {
		if existing.Name == call.Name {
			return false
		}
	}
	s.toolCalls = append(s.toolCalls, call)
	return true
}

// orDefaultFinish 结束原因兜底（协议 done 帧可能缺省 done_reason）.
// [EN] Default the finish reason (done frames may omit done_reason).
func orDefaultFinish(reason string) string {
	if reason == "" {
		return "stop"
	}
	return reason
}

// response 输出聚合后的完整响应.
// [EN] Emit the aggregated full response.
func (s *streamAggregator) response(model string) *llmx.Response {
	choice := llmx.Choice{FinishReason: s.finish}
	if s.textBuf.Len() > 0 {
		choice.Content = append(choice.Content, llmx.TextPart{Text: s.textBuf.String()})
	}
	choice.Content = append(choice.Content, partsOf(s.toolCalls)...)

	resp := &llmx.Response{Usage: s.usage, Model: model}
	if s.sawFrame {
		resp.Choices = []llmx.Choice{choice}
	}
	return resp
}

// partsOf 工具调用集合 → Part 集合.
// [EN] Tool calls to parts.
func partsOf(calls []llmx.ToolCallPart) []llmx.Part {
	if len(calls) == 0 {
		return nil
	}
	parts := make([]llmx.Part, 0, len(calls))
	for _, c := range calls {
		parts = append(parts, c)
	}
	return parts
}
