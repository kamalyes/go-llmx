/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-27 15:00:05
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-27 15:07:52
 * @FilePath: \go-llmx\memory\history.go
 * @Description: 持久化历史契约 —— History 接口与进程内实现.
 * 与 Memory（截断策略）正交：History 负责存哪，Memory 负责带多少；
 * sqlite 等重型后端由 adapters 子模块扩展
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"context"
	"sync"

	llmx "github.com/kamalyes/go-llmx"
)

// History 对话历史存储（持久化抽象）.
// [EN] Conversation history storage (persistence abstraction).
type History interface {
	// AddMessage 追加一条消息.
	// [EN] Append one message.
	AddMessage(ctx context.Context, msg llmx.Message) error

	// Messages 返回全部历史（返回副本）.
	// [EN] Return the full history (a copy).
	Messages(ctx context.Context) ([]llmx.Message, error)

	// Clear 清空历史.
	// [EN] Clear the history.
	Clear(ctx context.Context) error
}

// InMemoryHistory 进程内历史（并发安全；重启即失，持久化场景用数据库后端）.
// [EN] In-process history (concurrency-safe; use a DB backend to persist).
type InMemoryHistory struct {
	mu   sync.RWMutex
	msgs []llmx.Message
}

// NewHistory 构造进程内历史.
// [EN] Build an in-memory history.
func NewHistory() *InMemoryHistory {
	return &InMemoryHistory{}
}

// AddUserMessage 便捷追加用户消息.
// [EN] Append a user message.
func (h *InMemoryHistory) AddUserMessage(ctx context.Context, text string) error {
	return h.AddMessage(ctx, llmx.User(text))
}

// AddAIMessage 便捷追加助手消息.
// [EN] Append an assistant message.
func (h *InMemoryHistory) AddAIMessage(ctx context.Context, text string) error {
	return h.AddMessage(ctx, llmx.Assistant(text))
}

// AddMessage 实现 History.
// [EN] Implement History.
func (h *InMemoryHistory) AddMessage(ctx context.Context, msg llmx.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgs = append(h.msgs, msg)
	return nil
}

// Messages 实现 History（副本返回）.
// [EN] Implement History (returns a copy).
func (h *InMemoryHistory) Messages(ctx context.Context) ([]llmx.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]llmx.Message(nil), h.msgs...), nil
}

// Clear 实现 History.
// [EN] Implement History.
func (h *InMemoryHistory) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgs = nil
	return nil
}

// 编译期断言：实现 History 契约.
// [EN] Compile-time assertion.
var _ History = (*InMemoryHistory)(nil)
