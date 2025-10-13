/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 20:33:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 20:41:00
 * @FilePath: \go-llmx\chain\llmchain.go
 * @Description: LLMChain —— 提示词模板 → 模型直调链（无状态），
 * 最常用的单步编排单元
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/prompt"
)

// LLMChain 提示词模板 → 模型直调链（无状态）.
// [EN] Prompt-template → model chain (stateless).
type LLMChain struct {
	// Model 目标模型.
	// [EN] Target model.
	Model llmx.Model

	// Prompt 输入模板（nil 时输入原样作为用户消息）.
	// [EN] Input template (input passed through when nil).
	Prompt *prompt.Template

	// Options 每次调用的请求级选项.
	// [EN] Per-call options.
	Options []llmx.Option
}

// Run 实现 Chain.
// [EN] Implement Chain.
func (c *LLMChain) Run(ctx context.Context, input string) (string, error) {
	if c == nil || c.Model == nil {
		return "", fmt.Errorf("%w: LLMChain missing model", llmx.ErrInvalidRequest)
	}
	text := input
	if c.Prompt != nil {
		rendered, err := c.Prompt.Render(chainInput{Input: input})
		if err != nil {
			return "", err
		}
		text = rendered
	}
	resp, err := c.Model.GenerateContent(ctx, []llmx.Message{llmx.Text(llmx.RoleUser, text)}, c.Options...)
	if err != nil {
		return "", err
	}
	return llmx.FirstText(resp)
}
