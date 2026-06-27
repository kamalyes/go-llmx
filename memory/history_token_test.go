/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-27 21:31:10
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-27 21:36:19
 * @FilePath: \go-llmx\memory\history_token_test.go
 * @Description: History 与 TokenBuffer 测试 —— 存取语义/预算淘汰/并发安全.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"context"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory_AddAndList(t *testing.T) {
	h := NewHistory()
	ctx := context.Background()

	require.NoError(t, h.AddUserMessage(ctx, "hi"))
	require.NoError(t, h.AddAIMessage(ctx, "hello"))

	msgs, err := h.Messages(ctx)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, llmx.RoleUser, msgs[0].Role)
	assert.Equal(t, llmx.RoleAssistant, msgs[1].Role)
}

func TestHistory_Clear(t *testing.T) {
	h := NewHistory()
	ctx := context.Background()
	_ = h.AddUserMessage(ctx, "x")

	require.NoError(t, h.Clear(ctx))

	msgs, err := h.Messages(ctx)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestHistory_ReturnsCopy(t *testing.T) {
	h := NewHistory()
	ctx := context.Background()
	_ = h.AddUserMessage(ctx, "x")

	msgs, _ := h.Messages(ctx)
	msgs[0] = llmx.User("tampered")

	again, _ := h.Messages(ctx)
	assert.Equal(t, "x", again[0].String())
}

func TestHistory_CancelledContext(t *testing.T) {
	h := NewHistory()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.Error(t, h.AddMessage(ctx, llmx.User("x")))
	_, err := h.Messages(ctx)
	require.Error(t, err)
	require.Error(t, h.Clear(ctx))
}

func TestApproxTokens(t *testing.T) {
	assert.Equal(t, 1, ApproxTokens("abc"))       // 3 runes → 1
	assert.Equal(t, 1, ApproxTokens("abcd"))      // 4 runes → 1
	assert.Equal(t, 2, ApproxTokens("abcde"))     // 5 runes → 2（向上取整）
	assert.Equal(t, 0, ApproxTokens(""))
}

func TestTokenBuffer_EvictsOldestBeyondBudget(t *testing.T) {
	b := NewTokenBuffer(3) // 每条 "abcdefgh" 约 2 token

	b.Add(llmx.User("abcdefgh"))  // 2 token
	b.Add(llmx.Assistant("abcdefgh")) // +2 = 4 > 3 → 淘汰首条
	b.Add(llmx.User("abcdefgh"))  // +2 = 4 > 3 → 再淘汰

	msgs := b.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, llmx.RoleUser, msgs[0].Role)
}

func TestTokenBuffer_UnboundedWhenDisabled(t *testing.T) {
	b := NewTokenBuffer(0)

	for i := 0; i < 100; i++ {
		b.Add(llmx.User("some text"))
	}
	assert.Len(t, b.Messages(), 100)
}

func TestTokenBuffer_ConcurrentAccess(t *testing.T) {
	b := NewTokenBuffer(50)
	var wg sync.WaitGroup

	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Add(llmx.User("concurrent message"))
			_ = b.Messages()
		}()
	}
	wg.Wait()

	assert.NotPanics(t, func() { b.Add(llmx.User("final")) })
}
