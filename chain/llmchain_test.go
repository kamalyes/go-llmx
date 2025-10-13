/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 21:28:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 21:28:00
 * @FilePath: \go-llmx\chain\llmchain_test.go
 * @Description: LLMChain 测试 —— 模板渲染/直通/选项透传/错误路径
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/prompt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLMChain_WithPrompt(t *testing.T) {
	p, err := prompt.New("翻译成英文：{{.Input}}")
	require.NoError(t, err)

	f := llmx.NewFakeModel("Hello")
	c := &LLMChain{Model: f, Prompt: p}

	out, err := c.Run(context.Background(), "你好")
	require.NoError(t, err)
	assert.Equal(t, "Hello", out)

	// 模板渲染结果作为用户消息
	call := f.LastCall()
	assert.Equal(t, "翻译成英文：你好", call[0].Content[0].(llmx.TextPart).Text)
}

func TestLLMChain_NoPromptPassthrough(t *testing.T) {
	f := llmx.NewFakeModel("ok")
	c := &LLMChain{Model: f}

	out, err := c.Run(context.Background(), "原始输入")
	require.NoError(t, err)
	assert.Equal(t, "ok", out)
	assert.Equal(t, "原始输入", f.LastCall()[0].Content[0].(llmx.TextPart).Text)
}

func TestLLMChain_OptionsPassed(t *testing.T) {
	f := llmx.NewFakeModel("ok")
	c := &LLMChain{Model: f, Options: []llmx.Option{llmx.WithTemperature(0.1)}}

	_, err := c.Run(context.Background(), "q")
	require.NoError(t, err)
	assert.Equal(t, 0.1, f.CallOptions[0].Temperature)
}

func TestLLMChain_MissingModel(t *testing.T) {
	c := &LLMChain{}
	_, err := c.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestLLMChain_RenderError(t *testing.T) {
	p, err := prompt.New("{{.Count.Total}}")
	require.NoError(t, err)

	c := &LLMChain{Model: llmx.NewFakeModel("x"), Prompt: p}
	_, err = c.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}
