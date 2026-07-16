/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 19:32:17
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 19:32:17
 * @FilePath: \go-llmx\chain\router_test.go
 * @Description: 路由链测试 —— 意图判定转发/名字清洗/缺省兜底/
 * 空名字报错/清单稳定，fake 双模型零网络
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routeFake 按序应答模型（首个应答目的地名）.
// [EN] Sequential-reply model (first reply is a destination).
type routeFake struct {
	// replies 按序返回文本.
	// [EN] Sequential replies.
	replies []string

	// prompts 收到的提示词.
	// [EN] Prompts received.
	prompts []string
}

// GenerateContent 实现 Model.
// [EN] Implement Model.
func (m *routeFake) GenerateContent(_ context.Context, msgs []llmx.Message, _ ...llmx.Option) (*llmx.Response, error) {
	reply := ""
	if len(m.replies) > 0 {
		reply = m.replies[0]
		m.replies = m.replies[1:]
	}
	m.prompts = append(m.prompts, msgs[0].String())
	return &llmx.Response{
		Choices: []llmx.Choice{{Content: []llmx.Part{llmx.TextPart{Text: reply}}}},
	}, nil
}

// StreamGenerateContent 实现 Model（测试不触达）.
// [EN] Implement Model (unused here).
func (m *routeFake) StreamGenerateContent(_ context.Context, _ []llmx.Message, _ llmx.StreamHandler, _ ...llmx.Option) (*llmx.Response, error) {
	return &llmx.Response{}, nil
}

// stubChain 固定应答链（记录转发原文）.
// [EN] Fixed-reply chain (records the forwarded input).
type stubChain struct {
	// answer 固定应答.
	// [EN] Canned answer.
	answer string

	// inputs 收到的输入.
	// [EN] Inputs received.
	inputs []string
}

// Run 实现 Chain.
// [EN] Implement Chain.
func (c *stubChain) Run(_ context.Context, input string) (string, error) {
	c.inputs = append(c.inputs, input)
	return c.answer, nil
}

// newTestRouter 测试路由器（两个目的地）.
// [EN] A test router (two destinations).
func newTestRouter(reply string) (*Router, *stubChain, *stubChain) {
	translate := &stubChain{answer: "TRANSLATED"}
	summary := &stubChain{answer: "SUMMARIZED"}
	r := NewRouter(&routeFake{replies: []string{reply}}, map[string]Chain{
		"translate": translate,
		"summary":   summary,
	})
	return r, translate, summary
}

// TestRouter_DispatchesByIntent 验证意图判定转发对应链.
// [EN] Verify intent-based dispatch.
func TestRouter_DispatchesByIntent(t *testing.T) {
	r, translate, _ := newTestRouter("translate")

	out, err := r.Route(context.Background(), "hello world")
	require.NoError(t, err)
	assert.Equal(t, "TRANSLATED", out)
	require.Len(t, translate.inputs, 1)
	assert.Equal(t, "hello world", translate.inputs[0]) // 原文转发
}

// TestRouter_NormalizesName 验证返回名字清洗（引号/句点）.
// [EN] Verify returned-name normalization.
func TestRouter_NormalizesName(t *testing.T) {
	r, _, summary := newTestRouter(`"summary".`)

	out, err := r.Route(context.Background(), "long text")
	require.NoError(t, err)
	assert.Equal(t, "SUMMARIZED", out)
	require.Len(t, summary.inputs, 1)
}

// TestRouter_UnknownFallsBack 验证未知目的地回落缺省.
// [EN] Verify the unknown-destination fallback.
func TestRouter_UnknownFallsBack(t *testing.T) {
	r, _, summary := newTestRouter("nonexistent")
	r.WithDefault("summary")

	out, err := r.Route(context.Background(), "text")
	require.NoError(t, err)
	assert.Equal(t, "SUMMARIZED", out)
	assert.Len(t, summary.inputs, 1)
}

// TestRouter_UnknownErrors 验证未知目的地且无缺省报错.
// [EN] Verify unknown destination without fallback errors.
func TestRouter_UnknownErrors(t *testing.T) {
	r, _, _ := newTestRouter("nonexistent")

	_, err := r.Route(context.Background(), "text")
	assert.ErrorIs(t, err, llmx.ErrInvalidResponse)
}

// TestRouter_EmptyName 验证空目的地名报错.
// [EN] Verify empty destination errors.
func TestRouter_EmptyName(t *testing.T) {
	r, _, _ := newTestRouter("   ")

	_, err := r.Route(context.Background(), "text")
	assert.ErrorIs(t, err, llmx.ErrInvalidResponse)
}

// TestRouter_StablePrompt 验证清单排序缓存提示词稳定.
// [EN] Verify the sorted-list stable prompt.
func TestRouter_StablePrompt(t *testing.T) {
	// 同目的地集合不同 map 遍历序构造多次，提示词应一致
	// [EN] Same set, different map orders: prompts must match.
	var first string
	for i := 0; i < 8; i++ {
		m := &routeFake{replies: []string{"translate"}}
		r := NewRouter(m, map[string]Chain{
			"translate": &stubChain{},
			"summary":   &stubChain{},
			"qa":        &stubChain{},
		})
		_, err := r.Route(context.Background(), "x")
		require.NoError(t, err)
		if i == 0 {
			first = m.prompts[0]
			continue
		}
		assert.Equal(t, first, m.prompts[0])
	}
}

// TestRouter_MissingModel 验证缺模型/目的地报错.
// [EN] Verify missing model/destinations errors.
func TestRouter_MissingModel(t *testing.T) {
	r := NewRouter(nil, map[string]Chain{"a": &stubChain{}})
	_, err := r.Route(context.Background(), "x")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)

	r2 := NewRouter(&routeFake{}, nil)
	_, err = r2.Route(context.Background(), "x")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}
