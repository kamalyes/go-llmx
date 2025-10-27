/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 22:18:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-27 21:26:00
 * @FilePath: \go-llmx\memory\window.go
 * @Description: 滑动窗口记忆 —— 仅保留最近 K 条消息，
 * 控制长对话的上下文长度
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"sync"

	llmx "github.com/kamalyes/go-llmx"
)

// Window 滑动窗口记忆（仅保留最近 K 条）.
// [EN] Sliding-window memory keeping the last K messages.
type Window struct {
	mu   sync.RWMutex
	k    int
	msgs []llmx.Message
}

// NewWindow 构造窗口记忆（k<=0 时兜底为 1）.
// [EN] Build a window memory (k clamped to 1 when non-positive).
func NewWindow(k int) *Window {
	if k <= 0 {
		k = 1
	}
	return &Window{k: k}
}

// Messages 实现 Memory.
// [EN] Implement Memory.
func (w *Window) Messages() []llmx.Message {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return append([]llmx.Message(nil), w.msgs...)
}

// Add 实现 Memory（超出窗口丢弃最旧消息）.
// [EN] Implement Memory (drops the oldest on overflow).
func (w *Window) Add(messages ...llmx.Message) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.msgs = append(w.msgs, messages...)
	if overflow := len(w.msgs) - w.k; overflow > 0 {
		w.msgs = append([]llmx.Message(nil), w.msgs[overflow:]...)
	}
}

// Clear 实现 Memory.
// [EN] Implement Memory.
func (w *Window) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.msgs = nil
}
