/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 18:26:03
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 18:26:03
 * @FilePath: \go-llmx\memory\summary_test.go
 * @Description: 摘要记忆测试 —— 配对入队/Condense 压缩窗/摘要与近轮
 * 共存/失败回滚/幂等，fake model 弹固定摘要零网络
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"context"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// summaryFake 固定应答摘要模型.
// [EN] Fixed-reply summary model.
type summaryFake struct {
	// reply 返回的摘要文本.
	// [EN] The canned summary.
	reply string

	// err 返回的错误（非 nil 模拟压缩失败）.
	// [EN] Error to return (simulates failure).
	err error

	// prompts 收到的提示词.
	// [EN] Prompts received.
	prompts []string
}

// GenerateContent 实现 Model.
// [EN] Implement Model.
func (m *summaryFake) GenerateContent(_ context.Context, msgs []llmx.Message, _ ...llmx.Option) (*llmx.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.prompts = append(m.prompts, msgs[0].String())
	return &llmx.Response{
		Choices: []llmx.Choice{{Content: []llmx.Part{llmx.TextPart{Text: m.reply}}}},
	}, nil
}

// StreamGenerateContent 实现 Model（测试不触达）.
// [EN] Implement Model (unused here).
func (m *summaryFake) StreamGenerateContent(_ context.Context, _ []llmx.Message, _ llmx.StreamHandler, _ ...llmx.Option) (*llmx.Response, error) {
	return &llmx.Response{}, nil
}

// addTurn 辅助追加一轮.
// [EN] Helper: add one turn.
func addTurn(s *Summary, q, a string) {
	s.Add(llmx.User(q), llmx.Assistant(a))
}

// TestSummary_Pairing 验证 user/assistant 配对入队.
// [EN] Verify user/assistant pairing.
func TestSummary_Pairing(t *testing.T) {
	s := NewSummary(&summaryFake{}, 2)
	addTurn(s, "q1", "a1")
	addTurn(s, "q2", "a2")

	msgs := s.Messages()
	require.Len(t, msgs, 4)
	assert.Equal(t, "q1", msgs[0].String())
	assert.Equal(t, "a1", msgs[1].String())
}

// TestSummary_CondenseBeyondWindow 验证超窗轮压缩、近窗保留.
// [EN] Verify condensing beyond the kept window.
func TestSummary_CondenseBeyondWindow(t *testing.T) {
	m := &summaryFake{reply: "用户问了天气和交通。"}
	s := NewSummary(m, 1)
	addTurn(s, "天气如何", "晴")
	addTurn(s, "交通如何", "顺")
	addTurn(s, "明天呢", "雨")

	// 3 轮保 1：前 2 轮应被压缩
	// [EN] Keep 1 of 3: the first two condense.
	require.NoError(t, s.Condense(context.Background()))

	msgs := s.Messages()
	require.Len(t, msgs, 3) // system 摘要 + user/assistant 最后一轮
	assert.Equal(t, llmx.RoleSystem, msgs[0].Role)
	assert.Contains(t, msgs[0].String(), "用户问了天气和交通。")
	assert.Equal(t, "明天呢", msgs[1].String())
	assert.Equal(t, "雨", msgs[2].String())

	// 压缩提示词含旧轮与新轮对话
	// [EN] The condense prompt carries old and new turns.
	require.Len(t, m.prompts, 1)
	assert.Contains(t, m.prompts[0], "user: 天气如何")
	assert.Contains(t, m.prompts[0], "(none)") // 无旧摘要时缺省话术
}

// TestSummary_SecondCondenseMerges 验证二次压缩合并旧摘要.
// [EN] Verify the second condense merges the old summary.
func TestSummary_SecondCondenseMerges(t *testing.T) {
	m := &summaryFake{reply: "merged"}
	s := NewSummary(m, 1)
	addTurn(s, "q1", "a1")
	addTurn(s, "q2", "a2")
	require.NoError(t, s.Condense(context.Background()))

	addTurn(s, "q3", "a3")
	addTurn(s, "q4", "a4")
	require.NoError(t, s.Condense(context.Background()))

	require.Len(t, m.prompts, 2)
	assert.Contains(t, m.prompts[1], "merged") // 旧摘要注入二次压缩
	assert.Contains(t, m.prompts[1], "user: q3")
}

// TestSummary_CondenseFailureRollsBack 验证压缩失败回滚原文.
// [EN] Verify failure rollback.
func TestSummary_CondenseFailureRollsBack(t *testing.T) {
	m := &summaryFake{err: assert.AnError}
	s := NewSummary(m, 1)
	addTurn(s, "q1", "a1")
	addTurn(s, "q2", "a2")
	addTurn(s, "q3", "a3")

	err := s.Condense(context.Background())
	require.ErrorIs(t, err, assert.AnError)

	// 回滚：3 轮原文全在，无摘要
	// [EN] Rollback: all three turns verbatim, no summary.
	msgs := s.Messages()
	require.Len(t, msgs, 6)
	assert.Equal(t, "q1", msgs[0].String())
	assert.Equal(t, "a3", msgs[5].String())
}

// TestSummary_CondenseIdempotent 验证窗口内压缩幂等（无 LLM 调用）.
// [EN] Verify in-window condense is a no-op.
func TestSummary_CondenseIdempotent(t *testing.T) {
	m := &summaryFake{reply: "x"}
	s := NewSummary(m, 2)
	addTurn(s, "q1", "a1")
	addTurn(s, "q2", "a2")

	require.NoError(t, s.Condense(context.Background()))
	assert.Empty(t, m.prompts) // 未超窗不调 LLM
}

// TestSummary_NilModel 验证缺模型压缩报错.
// [EN] Verify a missing model errors.
func TestSummary_NilModel(t *testing.T) {
	s := NewSummary(nil, 1)
	addTurn(s, "q1", "a1")
	addTurn(s, "q2", "a2")
	err := s.Condense(context.Background())
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// TestSummary_Clear 验证清空.
// [EN] Verify Clear.
func TestSummary_Clear(t *testing.T) {
	s := NewSummary(&summaryFake{reply: "x"}, 1)
	addTurn(s, "q1", "a1")
	addTurn(s, "q2", "a2")
	require.NoError(t, s.Condense(context.Background()))
	require.NotEmpty(t, s.Messages())

	s.Clear()
	assert.Empty(t, s.Messages())
}

// TestSummary_OddAssistant 验证孤立 assistant 轮独立入队.
// [EN] Verify a lone assistant turn queues standalone.
func TestSummary_OddAssistant(t *testing.T) {
	s := NewSummary(&summaryFake{}, 2)
	s.Add(llmx.Assistant("solo"))

	msgs := s.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "solo", msgs[0].String())
}
