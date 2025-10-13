/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 21:01:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 21:01:00
 * @FilePath: \go-llmx\chain\conversation.go
 * @Description: ConversationChain —— 带会话记忆的对话链，
 * 历史 + 本轮输入 → 模型 → 记忆回写，替代 langchaingo ConversationChain
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/memory"
	"github.com/kamalyes/go-llmx/prompt"
)

// ConversationChain 带会话记忆的对话链（历史 + 本轮输入 → 模型 → 记忆回写）.
// [EN] Memory-backed conversation chain.
type ConversationChain struct {
	// Model 目标模型.
	// [EN] Target model.
	Model llmx.Model

	// Memory 会话记忆（必填）.
	// [EN] Conversation memory (required).
	Memory memory.Memory

	// Prompt 输入模板（nil 时输入原样）.
	// [EN] Input template (input passed through when nil).
	Prompt *prompt.Template

	// Options 每次调用的请求级选项.
	// [EN] Per-call options.
	Options []llmx.Option
}

// Run 实现 Chain（本轮 user 与 assistant 回答均写入记忆）.
// [EN] Implement Chain (both the user turn and assistant answer are memorized).
func (c *ConversationChain) Run(ctx context.Context, input string) (string, error) {
	if c == nil || c.Model == nil {
		return "", fmt.Errorf("%w: ConversationChain missing model", llmx.ErrInvalidRequest)
	}
	if c.Memory == nil {
		return "", fmt.Errorf("%w: ConversationChain missing memory", llmx.ErrInvalidRequest)
	}
	text := input
	if c.Prompt != nil {
		rendered, err := c.Prompt.Render(chainInput{Input: input})
		if err != nil {
			return "", err
		}
		text = rendered
	}
	messages := append(c.Memory.Messages(), llmx.User(text))
	resp, err := c.Model.GenerateContent(ctx, messages, c.Options...)
	if err != nil {
		return "", err
	}
	answer, err := llmx.FirstText(resp)
	if err != nil {
		return "", err
	}
	c.Memory.Add(llmx.User(text), llmx.Assistant(answer))
	return answer, nil
}
