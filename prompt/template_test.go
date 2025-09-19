/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-19 22:31:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-19 22:31:00
 * @FilePath: \go-llmx\prompt\template_test.go
 * @Description: 提示词模板测试 —— 编译/渲染/变量替换/错误路径
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package prompt

import (
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplate_Render(t *testing.T) {
	p, err := New("把这句话翻译成英文：{{.Text}}（语气：{{.Tone}}）")
	require.NoError(t, err)

	out, err := p.Render(map[string]any{"Text": "你好", "Tone": "正式"})
	require.NoError(t, err)
	assert.Equal(t, "把这句话翻译成英文：你好（语气：正式）", out)
}

func TestTemplate_RenderStruct(t *testing.T) {
	p, err := New("城市 {{.City}} 的天气如何？")
	require.NoError(t, err)

	out, err := p.Render(struct{ City string }{City: "北京"})
	require.NoError(t, err)
	assert.Equal(t, "城市 北京 的天气如何？", out)
}

func TestTemplate_NoVariables(t *testing.T) {
	p, err := New("固定提示词")
	require.NoError(t, err)

	out, err := p.Render(nil)
	require.NoError(t, err)
	assert.Equal(t, "固定提示词", out)
}

func TestTemplate_ParseError(t *testing.T) {
	_, err := New("{{.Broken")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestTemplate_ExecuteError(t *testing.T) {
	p, err := New("{{.Count.Total}}")
	require.NoError(t, err)

	// Count 为 int，取 Total 字段报错 → ErrInvalidRequest
	_, err = p.Render(map[string]any{"Count": 42})
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}
