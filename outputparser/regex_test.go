/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-19 21:07:39
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-19 21:17:32
 * @FilePath: \go-llmx\outputparser\regex_test.go
 * @Description: 正则解析器测试 —— 命名捕获组抽取/无匹配/非法表达式.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegex_NamedGroups(t *testing.T) {
	p := MustNewRegex(`answer:\s*(?P<answer>.+?)\s+confidence:\s*(?P<confidence>\d+)%`)

	got, err := p.Parse("answer: Paris confidence: 92%")
	require.NoError(t, err)
	assert.Equal(t, "Paris", got["answer"])
	assert.Equal(t, "92", got["confidence"])
}

func TestRegex_NoMatch(t *testing.T) {
	p := MustNewRegex(`(?P<value>\d+)`)

	_, err := p.Parse("no digits here")
	require.ErrorIs(t, err, ErrParse)
}

func TestRegex_InvalidExpression(t *testing.T) {
	_, err := NewRegex(`(?P<broken>[`)
	require.ErrorIs(t, err, ErrParse)
}

func TestRegex_MustNewRegexPanics(t *testing.T) {
	assert.Panics(t, func() { MustNewRegex(`(`) })
}

func TestRegex_UnnamedGroupsSkipped(t *testing.T) {
	// 未命名捕获组不出现在结果 map 中
	p := MustNewRegex(`(?P<named>x)(y)`)

	got, err := p.Parse("xy")
	require.NoError(t, err)
	assert.Equal(t, "x", got["named"])
	assert.NotContains(t, got, "")
	_, hasUnnamed := got["1"]
	assert.False(t, hasUnnamed)
}
