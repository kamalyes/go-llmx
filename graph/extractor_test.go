/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-15 20:51:32
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-15 21:08:59
 * @FilePath: \go-llmx\graph\extractor_test.go
 * @Description: 提取器单测 —— 行式协议解析容错、LLM 提取器
 * 与 FakeModel 的全链路（提示词注入与输出还原）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package graph

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	llmx "github.com/kamalyes/go-llmx"
)

// TestParseTriples 验证行式协议解析（含容错与去重）.
// [EN] Verify line-protocol parsing (tolerance + dedup).
func TestParseTriples(t *testing.T) {
	text := `
小明|工作于|鹅厂
小明 | 喜欢 | 篮球
空主体||值
畸形行
小明|工作于|鹅厂
|两边都|空
`
	triples := ParseTriples(text)
	require.Len(t, triples, 2)
	assert.Equal(t, Triple{Subject: "小明", Predicate: "工作于", Object: "鹅厂"}, triples[0])
	assert.Equal(t, Triple{Subject: "小明", Predicate: "喜欢", Object: "篮球"}, triples[1])

	assert.Empty(t, ParseTriples(""))
	assert.Empty(t, ParseTriples("无格式文本"))
}

// TestLLMExtractor 验证 LLM 提取全链路（FakeModel 应答行式协议）.
// [EN] Verify the LLM extractor end to end (FakeModel line-protocol reply).
func TestLLMExtractor(t *testing.T) {
	model := llmx.NewFakeModel("小明|工作于|鹅厂\n小明|喜欢|篮球")
	e := NewLLMExtractor(model)

	triples, err := e.Extract(context.Background(), "小明在鹅厂上班，喜欢打篮球")
	require.NoError(t, err)
	require.Len(t, triples, 2)
	assert.Equal(t, Triple{Subject: "小明", Predicate: "工作于", Object: "鹅厂"}, triples[0])

	messages := model.LastCall()
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].String(), "小明在鹅厂上班")
	assert.Contains(t, messages[0].String(), "主体|谓词|客体")
}

// TestLLMExtractor_EmptyReply 验证空应答容错.
// [EN] Verify tolerance of empty replies.
func TestLLMExtractor_EmptyReply(t *testing.T) {
	e := NewLLMExtractor(llmx.NewFakeModel("没有可抽取的三元组"))
	triples, err := e.Extract(context.Background(), "text")
	require.NoError(t, err)
	assert.Empty(t, triples)
}

// TestLLMExtractor_Error 验证模型错误透传.
// [EN] Verify model error propagation.
func TestLLMExtractor_Error(t *testing.T) {
	e := NewLLMExtractor(errModel{})
	_, err := e.Extract(context.Background(), "text")
	assert.ErrorIs(t, err, errBoom)
}

// errModel 恒错模型.
// [EN] An always-failing model.
type errModel struct{}

// errBoom 测试哨兵错误.
// [EN] Test sentinel error.
var errBoom = llmx.ErrInvalidRequest

// GenerateContent 实现 llmx.Model.
// [EN] Implement llmx.Model.
func (errModel) GenerateContent(context.Context, []llmx.Message, ...llmx.Option) (*llmx.Response, error) {
	return nil, errBoom
}

// StreamGenerateContent 实现 llmx.Model.
// [EN] Implement llmx.Model.
func (errModel) StreamGenerateContent(context.Context, []llmx.Message, llmx.StreamHandler, ...llmx.Option) (*llmx.Response, error) {
	return nil, errBoom
}

// TestTripleKey 验证去重键（含分隔符隔离防碰撞）.
// [EN] Verify the dedup key (separator isolation).
func TestTripleKey(t *testing.T) {
	a := Triple{Subject: "a", Predicate: "b", Object: "c"}
	b := Triple{Subject: "ab", Predicate: "", Object: "bc"}
	assert.NotEqual(t, a.key(), b.key())
	assert.True(t, strings.Contains(a.key(), "\x00"))
}
