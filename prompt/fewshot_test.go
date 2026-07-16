/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 15:02:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 15:12:08
 * @FilePath: \go-llmx\prompt\fewshot_test.go
 * @Description: 少样本模板测试 —— 长度预算/语义 top-K 选择、前后缀拼接、
 * 消息级模板渲染、错误路径；fake embedder 零网络依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package prompt

import (
	"context"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newExampleTpl 构造通用示例模板.
// [EN] Build the common example template.
func newExampleTpl(t *testing.T) *Template {
	tpl, err := New("输入：{{.Input}}\n输出：{{.Output}}")
	require.NoError(t, err)
	return tpl
}

// TestFewShot_RenderAll 验证无选择器全量注入与前后缀包裹.
// [EN] Verify full injection without a selector.
func TestFewShot_RenderAll(t *testing.T) {
	f := NewFewShot(newExampleTpl(t),
		Example{"Input": "1", "Output": "一"},
		Example{"Input": "2", "Output": "二"},
	).
		WithPrefix("以下是示例：").
		WithSuffix("现在请处理新输入。")

	out, err := f.Render(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "以下是示例：\n\n输入：1\n输出：一\n\n输入：2\n输出：二\n\n现在请处理新输入。", out)
}

// TestFewShot_EmptyPool 验证空示例池只渲染前后缀.
// [EN] Verify empty pool renders prefix/suffix only.
func TestFewShot_EmptyPool(t *testing.T) {
	f := NewFewShot(newExampleTpl(t)).
		WithPrefix("P").
		WithSuffix("S")

	out, err := f.Render(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "P\n\n\n\nS", out)
}

// TestFewShot_MissingTemplate 验证缺示例模板报错.
// [EN] Verify missing example template errors.
func TestFewShot_MissingTemplate(t *testing.T) {
	f := NewFewShot(nil, Example{"Input": "x"})
	_, err := f.Render(context.Background(), "")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// TestLengthSelector_Budget 验证预算先入先选与至少保一.
// [EN] Verify first-fit budgeting and the keep-one minimum.
func TestLengthSelector_Budget(t *testing.T) {
	// 每示例 4 rune = 1 token，预算 2 → 前两个入选
	// [EN] Each example is 4 runes; budget 2 keeps the first two.
	sel := NewLengthSelector(2)
	examples := []Example{
		{"Input": "aaaa"},
		{"Input": "bbbb"},
		{"Input": "cccc"},
	}
	got := sel.Select(context.Background(), "", examples)
	require.Len(t, got, 2)
	assert.Equal(t, "aaaa", got[0]["Input"])
	assert.Equal(t, "bbbb", got[1]["Input"])
}

// TestLengthSelector_Unbounded 验证零预算不限制.
// [EN] Verify zero budget is unbounded.
func TestLengthSelector_Unbounded(t *testing.T) {
	sel := NewLengthSelector(0)
	examples := make([]Example, 10)
	got := sel.Select(context.Background(), "", examples)
	assert.Len(t, got, 10)
}

// fakeEmbedder 确定性向量嵌入（关键词映射维度，无网络）.
// [EN] Deterministic keyword-vector embedder.
type fakeEmbedder struct{}

// EmbedDocuments 实现 Embedder（示例文本映射固定向量）.
// [EN] Implement Embedder.
func (fakeEmbedder) EmbedDocuments(_ context.Context, texts []string) ([][]float64, error) {
	vecs := make([][]float64, len(texts))
	for i, txt := range texts {
		vecs[i] = keywordVector(txt)
	}
	return vecs, nil
}

// EmbedQuery 实现 Embedder.
// [EN] Implement Embedder.
func (fakeEmbedder) EmbedQuery(_ context.Context, text string) ([]float64, error) {
	return keywordVector(text), nil
}

// keywordVector 确定性向量：文本长度与首字符决定方向.
// [EN] Deterministic vector keyed by length and first rune.
func keywordVector(text string) []float64 {
	if text == "" {
		return []float64{0, 0}
	}
	first := float64(text[0])
	return []float64{first, float64(len(text))}
}

// TestSemanticSelector_TopK 验证相似度排序与 top-K 截取.
// [EN] Verify similarity ordering and top-K truncation.
func TestSemanticSelector_TopK(t *testing.T) {
	sel := NewSemanticSelector(fakeEmbedder{}, 1)
	examples := []Example{
		{"Input": "apple"}, // 向量 (97,5)
		{"Input": "app"},   // 向量 (97,3)：与 "apple"(97,5) 同向分量大
		{"Input": "zebra"}, // 向量 (122,5)：方向偏
	}
	// 查询 "app" 与同名示例余弦为 1.0，top-1 选中自身
	// [EN] Query "app" matches the identical example at cosine 1.0.
	got := sel.Select(context.Background(), "app", examples)
	require.Len(t, got, 1)
	assert.Equal(t, "app", got[0]["Input"])
}

// TestSemanticSelector_BlankInput 验证空输入全量返回.
// [EN] Verify blank input returns all.
func TestSemanticSelector_BlankInput(t *testing.T) {
	sel := NewSemanticSelector(fakeEmbedder{}, 1)
	examples := []Example{{"Input": "a"}, {"Input": "b"}}
	got := sel.Select(context.Background(), "  ", examples)
	assert.Len(t, got, 2)
}

// TestFewShot_WithLengthSelector 验证选择器在渲染链路生效.
// [EN] Verify the selector works inside Render.
func TestFewShot_WithLengthSelector(t *testing.T) {
	f := NewFewShot(newExampleTpl(t),
		Example{"Input": "11", "Output": "一一"},
		Example{"Input": "22", "Output": "二二"},
		Example{"Input": "33", "Output": "三三"},
	).WithSelector(NewLengthSelector(3)) // 每示例 4 rune=1 token，预算 3 全入

	out, err := f.Render(context.Background(), "")
	require.NoError(t, err)
	assert.Contains(t, out, "输入：33")
}

// TestChatTemplate_Messages 验证消息级模板整段渲染.
// [EN] Verify whole-slice rendering.
func TestChatTemplate_Messages(t *testing.T) {
	sys, err := New("你是{{.Style}}风格的翻译助手")
	require.NoError(t, err)
	usr, err := New("翻译：{{.Text}}")
	require.NoError(t, err)

	ct, err := NewChat(System(sys), User(usr))
	require.NoError(t, err)

	msgs, err := ct.Messages(map[string]any{"Style": "正式", "Text": "hello"})
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, llmx.RoleSystem, msgs[0].Role)
	assert.Equal(t, "你是正式风格的翻译助手", msgs[0].String())
	assert.Equal(t, llmx.RoleUser, msgs[1].Role)
	assert.Equal(t, "翻译：hello", msgs[1].String())
}

// TestChatTemplate_Empty 验证空消息序列报错.
// [EN] Verify empty message list errors.
func TestChatTemplate_Empty(t *testing.T) {
	_, err := NewChat()
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}
