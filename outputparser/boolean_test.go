/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-18 20:05:35
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-18 20:12:53
 * @FilePath: \go-llmx\outputparser\boolean_test.go
 * @Description: 布尔解析器测试 —— 归一容错/自定义词表/失败路径.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoolean_ParseTrueVariants(t *testing.T) {
	p := NewBoolean()

	for _, text := range []string{"YES", "yes", " Yes ", "`true`", "\"TRUE\"", "\ntrue\n"} {
		v, err := p.Parse(text)
		require.NoError(t, err, "input: %q", text)
		assert.True(t, v, "input: %q", text)
	}
}

func TestBoolean_ParseFalseVariants(t *testing.T) {
	p := NewBoolean()

	for _, text := range []string{"NO", "no", " False ", "`FALSE`", "\"no\""} {
		v, err := p.Parse(text)
		require.NoError(t, err, "input: %q", text)
		assert.False(t, v, "input: %q", text)
	}
}

func TestBoolean_ParseInvalid(t *testing.T) {
	p := NewBoolean()

	_, err := p.Parse("maybe")
	require.ErrorIs(t, err, ErrParse)

	_, err = p.Parse("")
	require.ErrorIs(t, err, ErrParse)
}

func TestBoolean_CustomVocabulary(t *testing.T) {
	p := NewBoolean().WithVocabulary([]string{"是", "对的"}, []string{"否", "不对"})

	v, err := p.Parse(" 是 ")
	require.NoError(t, err)
	assert.True(t, v)

	v, err = p.Parse("不对")
	require.NoError(t, err)
	assert.False(t, v)

	// 原默认词表被完全替换
	_, err = p.Parse("YES")
	require.ErrorIs(t, err, ErrParse)
}

func TestBoolean_FormatInstructions(t *testing.T) {
	p := NewBoolean()

	assert.NotEmpty(t, p.GetFormatInstructions())
	assert.Contains(t, p.GetFormatInstructions(), "boolean")
}
