/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-21 20:39:23
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-21 20:58:02
 * @FilePath: \go-llmx\adapters\openai\thinking_test.go
 * @Description: OpenAI 思考模式测试 —— reasoning_effort 编码与请求形态.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"encoding/json"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openaiRequestFor(t *testing.T, opts ...llmx.Option) *wireRequest {
	t.Helper()
	c := New("test-key")
	req := c.buildRequest(llmx.Apply(opts...), nil, false)
	require.NotNil(t, req)
	return req
}

func TestReasoningEffort_NotCarriedByDefault(t *testing.T) {
	req := openaiRequestFor(t)

	assert.Empty(t, req.ReasoningEffort)
}

func TestReasoningEffort_LevelMapping(t *testing.T) {
	cases := []struct {
		mode llmx.ThinkingMode
		want string
	}{
		{llmx.ThinkingLow, "low"},
		{llmx.ThinkingMedium, "medium"},
		{llmx.ThinkingHigh, "high"},
	}
	for _, c := range cases {
		req := openaiRequestFor(t, llmx.WithThinkingMode(c.mode))
		assert.Equal(t, c.want, req.ReasoningEffort, "mode=%s", c.mode)
	}
}

func TestReasoningEffort_NoneAndAutoOmitted(t *testing.T) {
	for _, mode := range []llmx.ThinkingMode{llmx.ThinkingNone, llmx.ThinkingAuto} {
		req := openaiRequestFor(t, llmx.WithThinkingMode(mode))
		assert.Empty(t, req.ReasoningEffort, "mode=%s", mode)
	}
}

func TestReasoningEffort_BudgetIgnored(t *testing.T) {
	// OpenAI 协议无 budget 概念，仅档位映射
	req := openaiRequestFor(t, llmx.WithThinkingBudget(6000))

	assert.Empty(t, req.ReasoningEffort)
}

func TestReasoningEffort_JSONShape(t *testing.T) {
	req := openaiRequestFor(t, llmx.WithThinkingMode(llmx.ThinkingMedium))

	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"reasoning_effort":"medium"`)
}
