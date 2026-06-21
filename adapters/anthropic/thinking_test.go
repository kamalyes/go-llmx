/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-21 10:05:19
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-21 10:11:36
 * @FilePath: \go-llmx\adapters\anthropic\thinking_test.go
 * @Description: Anthropic 思考模式测试 —— 请求编码/预算钳制/协议参数形态.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcanthropic

import (
	"encoding/json"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildRequestFor 测试辅助：以指定选项构造 wire 请求.
// [EN] Test helper: build the wire request with the given options.
func buildRequestFor(t *testing.T, opts ...llmx.Option) *wireRequest {
	t.Helper()
	c := New("test-key")
	req := c.buildRequest(llmx.Apply(opts...), nil, false)
	require.NotNil(t, req)
	return req
}

func TestThinking_NotCarriedByDefault(t *testing.T) {
	req := buildRequestFor(t)

	assert.Nil(t, req.Thinking)
}

func TestThinking_ModeLevelDerivesBudget(t *testing.T) {
	req := buildRequestFor(t, llmx.WithThinkingMode(llmx.ThinkingMedium), llmx.WithMaxTokens(100000))

	require.NotNil(t, req.Thinking)
	assert.Equal(t, thinkingEnabled, req.Thinking.Type)
	assert.Equal(t, 50000, req.Thinking.BudgetTokens)
}

func TestThinking_ExplicitBudgetTakesPrecedence(t *testing.T) {
	req := buildRequestFor(t, llmx.WithThinkingBudget(8000), llmx.WithMaxTokens(100000))

	require.NotNil(t, req.Thinking)
	assert.Equal(t, 8000, req.Thinking.BudgetTokens)
}

func TestThinking_BudgetClampedToMaxTokens(t *testing.T) {
	// 显式预算超过 max_tokens 时钳制
	req := buildRequestFor(t, llmx.WithThinkingBudget(5000), llmx.WithMaxTokens(2000))

	require.NotNil(t, req.Thinking)
	assert.Equal(t, 2000, req.Thinking.BudgetTokens)
}

func TestThinking_FloorApplied(t *testing.T) {
	// 小预算档位被协议下限兜底
	req := buildRequestFor(t, llmx.WithThinkingMode(llmx.ThinkingLow), llmx.WithMaxTokens(2000))

	require.NotNil(t, req.Thinking)
	assert.Equal(t, llmx.MinThinkingBudget, req.Thinking.BudgetTokens)
}

func TestThinking_NoneModeOmitted(t *testing.T) {
	req := buildRequestFor(t, llmx.WithThinkingMode(llmx.ThinkingNone))

	assert.Nil(t, req.Thinking)
}

func TestThinking_JSONShape(t *testing.T) {
	// 未显式设置 MaxTokens 时 req 走 DefaultMaxTokens=4096，预算钳制至 4096
	req := buildRequestFor(t, llmx.WithThinkingBudget(6000), llmx.WithMaxTokens(100000))

	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"thinking":{"type":"enabled","budget_tokens":6000}`)
}

func TestThinking_DecodeReasoningBlocks(t *testing.T) {
	// 响应侧 thinking 块 → Choice.Reasoning（decodeChoice 已有路径，回归确认）
	wr := &wireResponse{
		Content: []wireBlock{
			{Type: blockTypeThinking, Thinking: "let me think..."},
			{Type: blockTypeText, Text: "The answer is 42."},
		},
		StopReason: stopReasonEndTurn,
	}
	choice := decodeChoice(wr)

	assert.Equal(t, "let me think...", choice.Reasoning)
	assert.Equal(t, "The answer is 42.", choice.Text())
}
