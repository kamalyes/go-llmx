/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 21:19:57
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 21:33:58
 * @FilePath: \go-llmx\adapters\mistral\stream.go
 * @Description: Mistral 流式增量聚合器 —— 文本增量透传，工具调用按 index 聚合
 * 参数拼接（与 openai 适配器同构，Mistral 无 reasoning 增量）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmistral

import (
	"encoding/json"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// streamAggregator 流式聚合状态机（单候选流）.
// [EN] Streaming aggregation state machine (single-choice stream).
type streamAggregator struct {
	// textBuf 文本累积（响应收口与空流判定）.
	// [EN] Text accumulation.
	textBuf strings.Builder

	// finish 结束原因.
	// [EN] Finish reason.
	finish string

	// usage 用量.
	// [EN] Usage.
	usage llmx.Usage

	// toolCalls 工具调用按调用序号聚合（Arguments 逐段拼接）.
	// [EN] Tool calls aggregated by call index.
	toolCalls map[int]*llmx.ToolCallPart

	// order 工具调用出现顺序（收口保序）.
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
	var wc wireStreamChunk
	if err := json.Unmarshal([]byte(data), &wc); err != nil {
		return nil // 非 JSON data 静默容忍
	}
	if wc.Usage != nil {
		s.usage = llmx.Usage{
			PromptTokens:     wc.Usage.PromptTokens,
			CompletionTokens: wc.Usage.CompletionTokens,
			TotalTokens:      wc.Usage.TotalTokens,
		}
	}

	for _, ch := range wc.Choices {
		d := ch.Delta
		if d.Content != "" {
			s.sawContent = true
			s.textBuf.WriteString(d.Content)
			if stream != nil {
				if err := stream(&llmx.Chunk{Content: d.Content}); err != nil {
					return err
				}
			}
		}
		for _, td := range d.ToolCalls {
			s.sawContent = true
			call, ok := s.toolCalls[td.Index]
			if !ok {
				call = &llmx.ToolCallPart{}
				s.toolCalls[td.Index] = call
				s.order = append(s.order, td.Index)
			}
			if td.ID != "" {
				call.ID = td.ID
			}
			if td.Function.Name != "" {
				call.Name = td.Function.Name
			}
			call.Arguments += td.Function.Arguments
			if stream != nil {
				delta := &llmx.ToolCallDelta{Index: td.Index, ID: td.ID, Name: td.Function.Name, Arguments: td.Function.Arguments}
				if err := stream(&llmx.Chunk{ToolCallDelta: delta}); err != nil {
					return err
				}
			}
		}
		if ch.FinishReason != "" {
			s.finish = ch.FinishReason
			s.sawContent = true
			if stream != nil {
				if err := stream(&llmx.Chunk{FinishReason: ch.FinishReason, Usage: s.usage}); err != nil {
					return err
				}
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
