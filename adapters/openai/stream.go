/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-01 20:39:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-01 20:39:00
 * @FilePath: \go-llmx\adapters\openai\stream.go
 * @Description: 流式增量聚合器 —— 消费 OpenAI 兼容 SSE 的 delta 帧：
 * 文本/思考轨迹逐段透传回调，工具调用按 index 聚合 Arguments 拼接，
 * 结束帧（finish_reason/usage）统一收口为 Chunk + 最终 Response
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"encoding/json"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// wireStreamDelta 单帧流式增量响应.
// [EN] A single streaming delta frame.
type wireStreamDelta struct {
	// Choices 候选增量（流式恒为单元素）.
	// [EN] Choice deltas (always single-element in streaming).
	Choices []struct {
		// Delta 本次增量内容.
		// [EN] Delta content of this frame.
		Delta struct {
			// Content 文本增量.
			// [EN] Text delta.
			Content string `json:"content"`

			// Reasoning 思考轨迹增量（DeepSeek reasoner）.
			// [EN] Reasoning trace delta.
			Reasoning string `json:"reasoning_content"`

			// ToolCalls 工具调用增量.
			// [EN] Tool call deltas.
			ToolCalls []wireToolUse `json:"tool_calls"`
		} `json:"delta"`

		// FinishReason 结束原因（仅结束帧携带）.
		// [EN] Finish reason (end frame only).
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`

	// Usage token 用量（多数 provider 仅最后一帧携带）.
	// [EN] Token usage (usually only the final frame).
	Usage *wireUsage `json:"usage"`
}

// streamAggregator 流式聚合状态机.
// [EN] Streaming aggregation state machine.
type streamAggregator struct {
	// textBuf 文本累积.
	// [EN] Text accumulation.
	textBuf strings.Builder

	// reasonBuf 思考轨迹累积.
	// [EN] Reasoning trace accumulation.
	reasonBuf strings.Builder

	// finish 结束原因.
	// [EN] Finish reason.
	finish string

	// usage 用量.
	// [EN] Usage.
	usage llmx.Usage

	// toolAgg 工具调用按 index 聚合（Arguments 逐段拼接）.
	// [EN] Tool calls aggregated by index.
	toolAgg map[int]*llmx.ToolCallPart

	// maxToolIndex 已见过的最大工具 index（防御网关非 0 起连续的序号）.
	// [EN] Max tool index seen (guards non-contiguous indices).
	maxToolIndex int

	// sawChoice 是否收到过任何 choice 帧（空流判定）.
	// [EN] Whether any choice frame was seen.
	sawChoice bool
}

// newStreamAggregator 构造聚合器.
// [EN] Build an aggregator.
func newStreamAggregator() *streamAggregator {
	return &streamAggregator{toolAgg: map[int]*llmx.ToolCallPart{}, maxToolIndex: -1}
}

// feed 消费一帧 SSE data 并透传增量给回调.
// [EN] Consume one SSE data frame and forward deltas to the callback.
//
// 非 JSON 帧静默容忍（部分网关注释/心跳以 data: 形态混入）
func (s *streamAggregator) feed(data string, stream llmx.StreamHandler) error {
	var d wireStreamDelta
	if err := json.Unmarshal([]byte(data), &d); err != nil {
		return nil
	}
	if d.Usage != nil {
		s.usage = llmx.Usage{
			PromptTokens:     d.Usage.PromptTokens,
			CompletionTokens: d.Usage.CompletionTokens,
			TotalTokens:      d.Usage.TotalTokens,
		}
	}

	for _, ch := range d.Choices {
		s.sawChoice = true
		if ch.FinishReason != "" {
			s.finish = ch.FinishReason
		}
		if ch.Delta.Content != "" {
			s.textBuf.WriteString(ch.Delta.Content)
			if err := stream(&llmx.Chunk{Content: ch.Delta.Content}); err != nil {
				return err
			}
		}
		if ch.Delta.Reasoning != "" {
			s.reasonBuf.WriteString(ch.Delta.Reasoning)
			if err := stream(&llmx.Chunk{Reasoning: ch.Delta.Reasoning}); err != nil {
				return err
			}
		}
		for _, tc := range ch.Delta.ToolCalls {
			agg, ok := s.toolAgg[tc.Index]
			if !ok {
				agg = &llmx.ToolCallPart{}
				s.toolAgg[tc.Index] = agg
			}
			if tc.Index > s.maxToolIndex {
				s.maxToolIndex = tc.Index
			}
			if tc.ID != "" {
				agg.ID = tc.ID
			}
			if tc.Function.Name != "" {
				agg.Name = tc.Function.Name
			}
			agg.Arguments += tc.Function.Arguments
			// 工具调用增量透传回调（Arguments 为本帧片段，非全量）
			if err := stream(&llmx.Chunk{ToolCallDelta: &llmx.ToolCallDelta{
				Index:     tc.Index,
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}}); err != nil {
				return err
			}
		}
	}

	// 结束帧：携带 finish_reason 或 usage 的完整信号
	if s.finish != "" {
		return stream(&llmx.Chunk{FinishReason: s.finish, Usage: s.usage})
	}
	return nil
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
	// 按 index 升序输出聚合后的工具调用（防御非 0 起连续序号）
	for i := 0; i <= s.maxToolIndex; i++ {
		if tc, ok := s.toolAgg[i]; ok {
			choice.Content = append(choice.Content, *tc)
		}
	}

	resp := &llmx.Response{Usage: s.usage, Model: model}
	if s.sawChoice || len(choice.Content) > 0 || choice.Reasoning != "" {
		resp.Choices = []llmx.Choice{choice}
	}
	return resp
}
