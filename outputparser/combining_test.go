/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-20 13:59:10
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-20 15:02:31
 * @FilePath: \go-llmx\outputparser\combining_test.go
 * @Description: 组合解析器测试 —— 按序回退/全部失败聚合/空列表防护.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// strictJSON 仅接受严格 ```json 块.
// [EN] Accepts only a strict ```json block.
type strictJSON struct{}

func (strictJSON) Parse(text string) (map[string]any, error) {
	return NewStructured(Field{Name: "v"}).Parse(text)
}

func (strictJSON) GetFormatInstructions() string { return "output ```json with field v" }

// looseAny 接受任意文本（兜底解析器）.
// [EN] Accepts any text (fallback parser).
type looseAny struct{}

func (looseAny) Parse(text string) (map[string]any, error) {
	return map[string]any{"v": text}, nil
}

func (looseAny) GetFormatInstructions() string { return "anything" }

func TestCombining_FirstSuccessWins(t *testing.T) {
	p := MustNewCombining[map[string]any](strictJSON{}, looseAny{})

	// JSON 形态走 strictJSON
	got, err := p.Parse("```json\n{\"v\": 1}\n```")
	require.NoError(t, err)
	assert.InDelta(t, 1, got["v"], 0.001)

	// 裸文本退化为 looseAny
	got, err = p.Parse("just plain text")
	require.NoError(t, err)
	assert.Equal(t, "just plain text", got["v"])
}

func TestCombining_AllFailed(t *testing.T) {
	failing := MustNewCombining[map[string]any](strictJSON{}, strictJSON{})

	_, err := failing.Parse("no json anywhere")
	require.ErrorIs(t, err, ErrParse)
	assert.Contains(t, err.Error(), "all parsers failed")
}

func TestCombining_EmptyParsers(t *testing.T) {
	_, err := NewCombining[map[string]any]()
	require.ErrorIs(t, err, ErrParse)

	assert.Panics(t, func() { MustNewCombining[bool]() })
}

func TestCombining_FormatInstructionsFromFirst(t *testing.T) {
	p := MustNewCombining[map[string]any](strictJSON{}, looseAny{})

	assert.Equal(t, "output ```json with field v", p.GetFormatInstructions())
}

func TestCombining_NilParserSkipped(t *testing.T) {
	p := MustNewCombining[map[string]any](nil, looseAny{})

	got, err := p.Parse("text")
	require.NoError(t, err)
	assert.Equal(t, "text", got["v"])
}
