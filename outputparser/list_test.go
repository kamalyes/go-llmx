/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-18 21:16:39
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-18 21:26:08
 * @FilePath: \go-llmx\outputparser\list_test.go
 * @Description: 列表解析器测试 —— 分割/去空白/空输入.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList_ParseBasic(t *testing.T) {
	p := NewList()

	got, err := p.Parse("foo, bar, baz")
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar", "baz"}, got)
}

func TestList_ParseTrimsWhitespace(t *testing.T) {
	p := NewList()

	got, err := p.Parse("  foo ,\tbar ,  baz  ")
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar", "baz"}, got)
}

func TestList_ParseSingle(t *testing.T) {
	p := NewList()

	got, err := p.Parse("only")
	require.NoError(t, err)
	assert.Equal(t, []string{"only"}, got)
}

func TestList_ParseEmpty(t *testing.T) {
	p := NewList()

	for _, text := range []string{"", "   ", "\n\t"} {
		got, err := p.Parse(text)
		require.NoError(t, err)
		assert.Empty(t, got)
		assert.NotNil(t, got)
	}
}

func TestList_FormatInstructions(t *testing.T) {
	p := NewList()

	assert.Contains(t, p.GetFormatInstructions(), "comma separated")
}
