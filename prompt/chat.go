/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 15:02:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 15:02:33
 * @FilePath: \go-llmx\prompt\chat.go
 * @Description: 消息级提示词模板 —— 角色序列整体渲染为 []llmx.Message，
 * 对齐 langchaingo ChatPromptTemplate 且零新依赖（text/template 复用）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package prompt

import (
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
)

// MessageTemplate 单条消息模板（角色 + 文本模板）.
// [EN] One message template (role + text template).
type MessageTemplate struct {
	// Role 消息角色（system/user/assistant/tool）.
	// [EN] Message role.
	Role llmx.Role

	// Template 文本模板（变量来自渲染数据）.
	// [EN] Text template (fed by render data).
	Template *Template
}

// System 构造 system 消息模板.
// [EN] Build a system message template.
func System(tpl *Template) MessageTemplate {
	return MessageTemplate{Role: llmx.RoleSystem, Template: tpl}
}

// User 构造 user 消息模板.
// [EN] Build a user message template.
func User(tpl *Template) MessageTemplate {
	return MessageTemplate{Role: llmx.RoleUser, Template: tpl}
}

// Assistant 构造 assistant 消息模板.
// [EN] Build an assistant message template.
func Assistant(tpl *Template) MessageTemplate {
	return MessageTemplate{Role: llmx.RoleAssistant, Template: tpl}
}

// ChatTemplate 消息序列模板：一次渲染产出整段对话前缀
// （system 设定 + few-shot 展开等场景）.
// [EN] Message-sequence template: one render yields a message slice.
type ChatTemplate struct {
	// messages 消息模板序列（渲染顺序即消息顺序）.
	// [EN] Ordered message templates.
	messages []MessageTemplate
}

// NewChat 构造消息序列模板（至少一条消息模板）.
// [EN] Build a chat template (at least one message).
func NewChat(messages ...MessageTemplate) (*ChatTemplate, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("%w: chat template requires at least one message", llmx.ErrInvalidRequest)
	}
	for i, m := range messages {
		if m.Template == nil {
			return nil, fmt.Errorf("%w: message %d has no template", llmx.ErrInvalidRequest, i)
		}
	}
	return &ChatTemplate{messages: messages}, nil
}

// Messages 以数据渲染整段消息（每条模板共享同一数据载体）.
// [EN] Render the whole message slice with one data carrier.
func (c *ChatTemplate) Messages(data any) ([]llmx.Message, error) {
	out := make([]llmx.Message, 0, len(c.messages))
	for _, m := range c.messages {
		text, err := m.Template.Render(data)
		if err != nil {
			return nil, err
		}
		out = append(out, llmx.Text(m.Role, text))
	}
	return out, nil
}
