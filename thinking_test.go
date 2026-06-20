/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-20 21:22:01
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-20 21:29:17
 * @FilePath: \go-llmx\thinking_test.go
 * @Description: 思考模式测试 —— 档位预算推导/模型能力探测/选项注入.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateThinkingBudget_Ratios(t *testing.T) {
	cases := []struct {
		mode      ThinkingMode
		maxTokens int
		want      int
	}{
		{ThinkingLow, 100000, 20000},
		{ThinkingMedium, 100000, 50000},
		{ThinkingHigh, 100000, 80000},
		{ThinkingAuto, 100000, 80000},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, CalculateThinkingBudget(c.mode, c.maxTokens), "mode=%s", c.mode)
	}
}

func TestCalculateThinkingBudget_FloorAndFallback(t *testing.T) {
	// 小预算被协议下限兜底
	assert.Equal(t, MinThinkingBudget, CalculateThinkingBudget(ThinkingLow, 100))
	// 未设置 MaxTokens 走 8192 兜底再按比例（8192 * 0.2 = 1638）
	assert.Equal(t, 1638, CalculateThinkingBudget(ThinkingLow, 0))
	// none 档位不产生预算
	assert.Equal(t, 0, CalculateThinkingBudget(ThinkingNone, 100000))
}

func TestSupportsReasoning_KnownPrefixes(t *testing.T) {
	for _, m := range []string{
		"o1-preview", "o1", "o3-mini", "o4-mini",
		"claude-3-7-sonnet", "claude-sonnet-4-5", "claude-opus-4-1",
		"deepseek-r1", "deepseek-reasoner",
		"grok-3-mini", "qwq-32b",
	} {
		assert.True(t, SupportsReasoning(m), "model=%s", m)
	}
}

func TestSupportsReasoning_Negative(t *testing.T) {
	for _, m := range []string{"", "gpt-4o", "gpt-4o-mini", "claude-3-haiku", "llama3", "gemini-2.0-flash"} {
		assert.False(t, SupportsReasoning(m), "model=%s", m)
	}
}

func TestSupportsReasoning_SuffixFallback(t *testing.T) {
	assert.True(t, SupportsReasoning("my-model-thinking"))
	assert.True(t, SupportsReasoning("custom-reasoner"))
}

func TestWithThinkingMode_Option(t *testing.T) {
	o := Apply(WithThinkingMode(ThinkingHigh))

	require.NotNil(t, o.Thinking)
	assert.Equal(t, ThinkingHigh, o.Thinking.Mode)
	assert.Equal(t, 0, o.Thinking.BudgetTokens)
}

func TestWithThinkingBudget_Option(t *testing.T) {
	o := Apply(WithThinkingBudget(5000))

	require.NotNil(t, o.Thinking)
	assert.Equal(t, 5000, o.Thinking.BudgetTokens)

	// 负值归零
	o = Apply(WithThinkingBudget(-3))
	assert.Equal(t, 0, o.Thinking.BudgetTokens)
}

func TestThinking_DefaultNil(t *testing.T) {
	o := Apply()

	assert.Nil(t, o.Thinking)
}
