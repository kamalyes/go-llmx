/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-28 10:16:19
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-28 10:23:06
 * @FilePath: \go-llmx\memory\tokenbuffer.go
 * @Description: Token 预算缓冲记忆 —— 按近似 token 数淘汰最老消息.
 * 近似规则 rune/4（英文 ~4 字符/token）；不引入 tiktoken 依赖，
 * 精确预算场景请走 provider Usage 回填
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"sync"

	llmx "github.com/kamalyes/go-llmx"
)

// 近似换算：每 token 的字符数.
// [EN] Approximate chars per token.
const approxCharsPerToken = 4

// TokenBuffer token 预算缓冲记忆（近似计数；超预算从最老开始淘汰）.
// [EN] Token-budgeted buffer memory (approximate; oldest evicted first).
type TokenBuffer struct {
	mu        sync.RWMutex
	msgs      []llmx.Message
	maxTokens int
}

// NewTokenBuffer 构造 token 预算缓冲（maxTokens <=0 时不限）.
// [EN] Build a token buffer (unbounded when maxTokens <= 0).
func NewTokenBuffer(maxTokens int) *TokenBuffer {
	return &TokenBuffer{maxTokens: maxTokens}
}

// ApproxTokens 近似 token 计数（导出供调用方复用同一规则）.
// [EN] Approximate token count (exported for consistency).
func ApproxTokens(text string) int {
	n := len([]rune(text))
	return (n + approxCharsPerToken - 1) / approxCharsPerToken
}

// Messages 实现 Memory（预算内历史副本）.
// [EN] Implement Memory.
func (b *TokenBuffer) Messages() []llmx.Message {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]llmx.Message(nil), b.msgs...)
}

// Add 实现 Memory（追加后立即淘汰超出预算的最老消息）.
// [EN] Implement Memory (evicts oldest beyond budget).
func (b *TokenBuffer) Add(messages ...llmx.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, messages...)
	if b.maxTokens <= 0 {
		return
	}
	// 从最老开始淘汰直至预算内
	for b.totalTokens() > b.maxTokens && len(b.msgs) > 0 {
		b.msgs = b.msgs[1:]
	}
}

// Clear 实现 Memory.
// [EN] Implement Memory.
func (b *TokenBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = nil
}

// totalTokens 当前全部消息近似 token 总数（须持锁调用）.
// [EN] Total approximate tokens (must hold the lock).
func (b *TokenBuffer) totalTokens() int {
	total := 0
	for _, m := range b.msgs {
		total += ApproxTokens(m.String())
	}
	return total
}

// 编译期断言：实现 Memory 契约.
// [EN] Compile-time assertion.
var _ Memory = (*TokenBuffer)(nil)
