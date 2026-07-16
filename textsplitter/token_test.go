/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 21:18:26
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 21:26:33
 * @FilePath: \go-llmx\textsplitter\token_test.go
 * @Description: 近似 token 分块器测试 —— 预算切块/词边界优先/重叠衔接/
 * 空文本/ApproxTokens 换算，纯内存零依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package textsplitter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenSplitter_Budget 验证块 token 预算不超限.
// [EN] Verify the chunk token budget.
func TestTokenSplitter_Budget(t *testing.T) {
	s := NewTokenSplitter(10, 0) // 每块 ≈40 rune
	text := strings.Repeat("word ", 30)
	for _, chunk := range s.Split(text) {
		assert.LessOrEqual(t, ApproxTokens(chunk), 10)
	}
}

// TestTokenSplitter_Coverage 验证切块整体覆盖原文（overlap 语义下词必现）.
// [EN] Verify chunks collectively cover the source.
func TestTokenSplitter_Coverage(t *testing.T) {
	s := NewTokenSplitter(8, 2)
	text := "The quick brown fox jumps over the lazy dog. " +
		"Pack my box with five dozen liquor jugs. " +
		"How vexingly quick daft zebras jump!"
	chunks := s.Split(text)
	require.NotEmpty(t, chunks)

	// 每个原文词至少出现在一个块中
	// [EN] Every source word appears in some chunk.
	for _, w := range strings.Fields(text) {
		found := false
		for _, c := range chunks {
			if strings.Contains(c, w) {
				found = true
				break
			}
		}
		assert.True(t, found, "词 %q 丢失", w)
	}
}

// TestTokenSplitter_WordBoundary 验证切点不在词中间（无 overlap 场景）.
// [EN] Verify cuts land on word boundaries.
func TestTokenSplitter_WordBoundary(t *testing.T) {
	s := NewTokenSplitter(4, 0)
	chunks := s.Split("alpha beta gamma delta epsilon zeta eta theta")
	require.Greater(t, len(chunks), 1)
	for _, c := range chunks {
		assert.False(t, strings.HasPrefix(c, " "), "块不应以空白开头")
	}
}

// TestTokenSplitter_Overlap 验证相邻块存在重叠衔接.
// [EN] Verify adjacent chunks overlap.
func TestTokenSplitter_Overlap(t *testing.T) {
	s := NewTokenSplitter(5, 2) // 块 ≈20 rune，重叠 ≈8 rune
	text := strings.Repeat("seg ", 40)
	chunks := s.Split(text)
	require.Greater(t, len(chunks), 2)

	// 相邻块应共享内容（重叠非空）
	// [EN] Adjacent chunks must share content.
	for i := 1; i < len(chunks); i++ {
		prev, cur := chunks[i-1], chunks[i]
		shared := false
		for _, w := range strings.Fields(prev) {
			if strings.Contains(cur, w) {
				shared = true
				break
			}
		}
		assert.True(t, shared, "相邻块 %d 无重叠", i)
	}
}

// TestTokenSplitter_Empty 验证空文本返回空.
// [EN] Verify empty text yields nothing.
func TestTokenSplitter_Empty(t *testing.T) {
	s := NewTokenSplitter(10, 0)
	assert.Empty(t, s.Split(""))
	assert.Empty(t, s.Split("   "))
}

// TestTokenSplitter_Clamps 验证非法参数归一.
// [EN] Verify parameter clamping.
func TestTokenSplitter_Clamps(t *testing.T) {
	s := NewTokenSplitter(0, -1)
	assert.Equal(t, 512, s.chunkTokens)
	assert.Equal(t, 0, s.overlapTokens)

	s2 := NewTokenSplitter(10, 99) // overlap >= chunk → 块/4
	assert.Equal(t, 2, s2.overlapTokens)
}

// TestApproxTokens 验证近似换算（向上取整）.
// [EN] Verify the approximation (rounds up).
func TestApproxTokens(t *testing.T) {
	assert.Equal(t, 0, ApproxTokens(""))
	assert.Equal(t, 1, ApproxTokens("abcd"))  // 4 rune → 1
	assert.Equal(t, 2, ApproxTokens("abcde")) // 5 rune → 向上取 2
	assert.Equal(t, 3, ApproxTokens("一二三四五六七八九十")) // CJK 10 rune → 3
}
