/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-01 21:39:26
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-01 21:52:36
 * @FilePath: \go-llmx\adapters\googleai\stream.go
 * @Description: Gemini 流式增量聚合器 —— 每帧为完整 wireResponse 结构：
 * 文本 parts 逐帧透传，functionCall 整帧收集（args 一次到位不分片），
 * usageMetadata 取末帧收口
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgoogleai

import (
	"encoding/json"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/transport"
)

// streamAggregator 流式聚合状态机.
// [EN] Streaming aggregation state machine.
type streamAggregator struct {
	// choice 单候选累积（Gemini 主打单候选流）.
	// [EN] Single-candidate accumulation.
	choice llmx.Choice

	// usage 用量（末帧覆盖）.
	// [EN] Usage (last frame wins).
	usage llmx.Usage

	// sawContent 是否收到过任何内容信号（空流判定）.
	// [EN] Whether any content signal was seen.
	sawContent bool
}

// newStreamAggregator 构造聚合器.
// [EN] Build an aggregator.
func newStreamAggregator() *streamAggregator {
	return &streamAggregator{}
}

// feed 消费一帧 SSE 事件并透传增量给回调.
// [EN] Consume one SSE event and forward deltas to the callback.
//
// 每帧为完整 wireResponse：parts 为本轮全量片段（新 parts 直接透传），
// usageMetadata 取末帧，finishReason 到达即表示该候选收口
func (s *streamAggregator) feed(ev transport.SSEEvent, stream llmx.StreamHandler) error {
	var wr wireResponse
	if err := json.Unmarshal([]byte(ev.Data), &wr); err != nil {
		return nil // 非 JSON data 静默容忍（网关注释/心跳混入）
	}
	if wr.UsageMetadata != nil {
		s.usage = llmx.Usage{
			PromptTokens:     wr.UsageMetadata.PromptTokenCount,
			CompletionTokens: wr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      wr.UsageMetadata.TotalTokenCount,
		}
	}

	for _, cand := range wr.Candidates {
		var textDelta string
		var sawToolCall bool
		if cand.Content != nil {
			for _, p := range cand.Content.Parts {
				switch {
				case p.Text != "":
					textDelta += p.Text
					s.choice.Content = append(s.choice.Content, llmx.TextPart{Text: p.Text})
				case p.FunctionCall != nil:
					sawToolCall = true
					s.choice.Content = append(s.choice.Content, llmx.ToolCallPart{
						ID:        p.FunctionCall.Name,
						Name:      p.FunctionCall.Name,
						Arguments: rawToJSONString(p.FunctionCall.Args),
					})
				}
			}
		}
		if cand.FinishReason != "" {
			s.choice.FinishReason = mapFinishReason(cand.FinishReason, cand.Content)
		}
		if textDelta != "" || sawToolCall || cand.FinishReason != "" {
			s.sawContent = true
			if stream != nil {
				chunk := &llmx.Chunk{Content: textDelta, FinishReason: s.choice.FinishReason}
				if sawToolCall && s.choice.FinishReason == "" {
					chunk.FinishReason = "tool_calls"
				}
				if err := stream(chunk); err != nil {
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
	if s.sawContent {
		resp.Choices = []llmx.Choice{s.choice}
	}
	return resp
}
