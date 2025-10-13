/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 21:51:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 21:51:00
 * @FilePath: \go-llmx\chain\conversation_test.go
 * @Description: ConversationChain 测试 —— 记忆读写回路/错误路径/空响应
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/memory"
	"github.com/kamalyes/go-llmx/prompt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationChain_MemoryRoundtrip(t *testing.T) {
	p, err := prompt.New("用户说：{{.Input}}")
	require.NoError(t, err)

	f := llmx.NewFakeModel("第一答", "第二答")
	mem := memory.NewBuffer()
	c := &ConversationChain{Model: f, Memory: mem, Prompt: p}

	out, err := c.Run(context.Background(), "一")
	require.NoError(t, err)
	assert.Equal(t, "第一答", out)

	// 记忆写入 user（渲染后）+ assistant
	msgs := mem.Messages()
	require.Len(t, msgs, 2)
	assert.Equal(t, "用户说：一", msgs[0].Content[0].(llmx.TextPart).Text)
	assert.Equal(t, "第一答", msgs[1].Content[0].(llmx.TextPart).Text)

	// 第二轮携带历史
	out, err = c.Run(context.Background(), "二")
	require.NoError(t, err)
	assert.Equal(t, "第二答", out)
	call := f.LastCall()
	require.Len(t, call, 3)
	assert.Equal(t, "用户说：二", call[2].Content[0].(llmx.TextPart).Text)
}

func TestConversationChain_MissingParts(t *testing.T) {
	_, err := (&ConversationChain{}).Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)

	_, err = (&ConversationChain{Model: llmx.NewFakeModel("x")}).Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestConversationChain_RenderError(t *testing.T) {
	p, err := prompt.New("{{.Count.Total}}")
	require.NoError(t, err)

	c := &ConversationChain{Model: llmx.NewFakeModel("x"), Memory: memory.NewBuffer(), Prompt: p}
	_, err = c.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestConversationChain_ModelError(t *testing.T) {
	f := &llmx.FakeModel{Err: llmx.ErrProviderUnavailable}
	c := &ConversationChain{Model: f, Memory: memory.NewBuffer()}
	_, err := c.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)
}

func TestConversationChain_EmptyAnswer(t *testing.T) {
	// 模型返回空 choices → FirstText 报 ErrEmptyResponse，记忆不回写
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{{}}
	mem := memory.NewBuffer()
	c := &ConversationChain{Model: f, Memory: mem}
	_, err := c.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
	assert.Empty(t, mem.Messages())
}
