/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 22:07:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 22:07:00
 * @FilePath: \go-llmx\memory\buffer.go
 * @Description: 全量缓冲记忆 —— 无界保存完整对话历史，
 * 适合简单对话场景；上下文敏感场景用 window.go 滑窗
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"sync"

	llmx "github.com/kamalyes/go-llmx"
)

// Buffer 全量缓冲记忆（无界）.
// [EN] Unbounded buffer memory.
type Buffer struct {
	mu   sync.RWMutex
	msgs []llmx.Message
}

// NewBuffer 构造全量缓冲记忆.
// [EN] Build a buffer memory.
func NewBuffer() *Buffer {
	return &Buffer{}
}

// Messages 实现 Memory.
// [EN] Implement Memory.
func (b *Buffer) Messages() []llmx.Message {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]llmx.Message(nil), b.msgs...)
}

// Add 实现 Memory.
// [EN] Implement Memory.
func (b *Buffer) Add(messages ...llmx.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, messages...)
}

// Clear 实现 Memory.
// [EN] Implement Memory.
func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = nil
}
