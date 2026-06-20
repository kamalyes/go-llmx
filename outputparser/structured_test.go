/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-20 10:19:18
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-20 10:25:58
 * @FilePath: \go-llmx\outputparser\structured_test.go
 * @Description: 结构化解析器测试 —— JSON 块提取/必填校验/格式指令渲染.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func structuredFixture() *Structured {
	return NewStructured(
		Field{Name: "name", Description: "person name"},
		Field{Name: "age", Description: "age in years", Type: "number"},
	)
}

func TestStructured_ParseJSONBlock(t *testing.T) {
	p := structuredFixture()

	text := "Here is the result:\n```json\n{\"name\": \"Alice\", \"age\": 30}\n```\nhope this helps"
	got, err := p.Parse(text)
	require.NoError(t, err)
	assert.Equal(t, "Alice", got["name"])
	assert.InDelta(t, 30, got["age"], 0.001)
}

func TestStructured_ParseMissingFields(t *testing.T) {
	p := structuredFixture()

	_, err := p.Parse("```json\n{\"name\": \"Alice\"}\n```")
	require.ErrorIs(t, err, ErrParse)
	assert.Contains(t, err.Error(), "age")
}

func TestStructured_ParseNoBlock(t *testing.T) {
	p := structuredFixture()

	_, err := p.Parse("plain text without json")
	require.ErrorIs(t, err, ErrParse)
}

func TestStructured_ParseUnterminatedBlock(t *testing.T) {
	p := structuredFixture()

	_, err := p.Parse("```json\n{\"name\": \"Alice\"")
	require.ErrorIs(t, err, ErrParse)
}

func TestStructured_ParseInvalidJSON(t *testing.T) {
	p := structuredFixture()

	_, err := p.Parse("```json\n{name: Alice}\n```")
	require.ErrorIs(t, err, ErrParse)
}

func TestStructured_FormatInstructions(t *testing.T) {
	p := structuredFixture()

	ins := p.GetFormatInstructions()
	assert.Contains(t, ins, "```json")
	assert.Contains(t, ins, `"name"`)
	assert.Contains(t, ins, "person name")
	assert.Contains(t, ins, `"age"`)
	assert.Contains(t, ins, "number")
}

func TestStructured_TypeDefaultsToString(t *testing.T) {
	p := NewStructured(Field{Name: "only"})

	assert.Contains(t, p.GetFormatInstructions(), `"only": string`)
}
