/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 21:37:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 21:37:00
 * @FilePath: \go-llmx\chain\sequential_test.go
 * @Description: SequentialChain 测试 —— 串联传递/空链/nil 元素/错误传播
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequentialChain(t *testing.T) {
	f1 := llmx.NewFakeModel("第一段输出")
	f2 := llmx.NewFakeModel("第二段输出")
	c1 := &LLMChain{Model: f1}
	c2 := &LLMChain{Model: f2}

	s := NewSequentialChain(c1, c2)
	out, err := s.Run(context.Background(), "起点")
	require.NoError(t, err)
	assert.Equal(t, "第二段输出", out)

	// 前一链输出作为后一链输入
	assert.Equal(t, "第一段输出", f2.LastCall()[0].Content[0].(llmx.TextPart).Text)
}

func TestSequentialChain_Empty(t *testing.T) {
	s := NewSequentialChain()
	out, err := s.Run(context.Background(), "透传")
	require.NoError(t, err)
	assert.Equal(t, "透传", out)
}

func TestSequentialChain_NilElement(t *testing.T) {
	s := NewSequentialChain(&LLMChain{Model: llmx.NewFakeModel("x")}, nil)
	_, err := s.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestSequentialChain_NilReceiver(t *testing.T) {
	var s *SequentialChain
	_, err := s.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestSequentialChain_ErrorPropagation(t *testing.T) {
	f := &llmx.FakeModel{Err: llmx.ErrProviderUnavailable}
	s := NewSequentialChain(&LLMChain{Model: f})
	_, err := s.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)
}
