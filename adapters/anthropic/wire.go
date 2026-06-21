/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 21:18:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-21 10:11:36
 * @FilePath: \go-llmx\adapters\anthropic\wire.go
 * @Description: Anthropic Messages 协议编解码 —— wire 请求/响应结构与 llmx 类型互转.
 * 协议差异收口：system 走顶层参数、tool_result 走 user 消息块、max_tokens 必填
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcanthropic

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// ============================================================================
// 请求结构
// ============================================================================

// wireRequest Messages 请求体.
// [EN] Messages request body.
type wireRequest struct {
	// Model 模型名.
	// [EN] Model name.
	Model string `json:"model"`

	// MaxTokens 输出 token 上限（协议必填）.
	// [EN] Max output tokens (required by the protocol).
	MaxTokens int `json:"max_tokens"`

	// System 系统提示词（顶层参数，非消息列表）.
	// [EN] System prompt (top-level, not in the message list).
	System string `json:"system,omitempty"`

	// Messages 对话消息（仅 user/assistant 两种角色）.
	// [EN] Conversation messages (user/assistant only).
	Messages []wireMessage `json:"messages"`

	// Temperature 采样温度.
	// [EN] Sampling temperature.
	Temperature float64 `json:"temperature,omitempty"`

	// TopP 核采样阈值.
	// [EN] Nucleus sampling threshold.
	TopP float64 `json:"top_p,omitempty"`

	// Stop 停止序列.
	// [EN] Stop sequences.
	Stop []string `json:"stop_sequences,omitempty"`

	// Stream 流式开关.
	// [EN] Streaming flag.
	Stream bool `json:"stream,omitempty"`

	// Tools 工具定义.
	// [EN] Tool definitions.
	Tools []wireTool `json:"tools,omitempty"`

	// Thinking 思考配置（extended thinking；nil 不携带）.
	// [EN] Thinking configuration (extended thinking; nil omits it).
	Thinking *wireThinking `json:"thinking,omitempty"`
}

// wireThinking 思考请求参数（Anthropic extended thinking 协议）.
// [EN] Thinking request parameter (Anthropic extended thinking protocol).
type wireThinking struct {
	// Type 固定 enabled.
	// [EN] Fixed to "enabled".
	Type string `json:"type"`

	// BudgetTokens 思考 token 预算（协议要求 >= 1024）.
	// [EN] Thinking token budget (protocol requires >= 1024).
	BudgetTokens int `json:"budget_tokens"`
}

// wireMessage 协议消息.
// [EN] Protocol message.
type wireMessage struct {
	// Role 角色（user/assistant；tool 结果编码为 user 消息内的 tool_result 块）.
	// [EN] Role (user/assistant; tool results as tool_result blocks in user messages).
	Role string `json:"role"`

	// Content 内容（string 纯文本 或 []wireBlock 多块）.
	// [EN] Content (string for plain text, or block array).
	Content any `json:"content"`
}

// wireBlock 内容块（文本/图片/工具调用/工具结果/思考轨迹）.
// [EN] Content block (text/image/tool_use/tool_result/thinking).
type wireBlock struct {
	// Type 块类型.
	// [EN] Block type.
	Type string `json:"type"`

	// Text 文本内容（text/thinking 块）.
	// [EN] Text content (text blocks).
	Text string `json:"text,omitempty"`

	// Thinking 思考轨迹内容（thinking 块）.
	// [EN] Thinking trace (thinking blocks).
	Thinking string `json:"thinking,omitempty"`

	// Source 图片源（image 块）.
	// [EN] Image source (image blocks).
	Source *wireImageSource `json:"source,omitempty"`

	// ID 工具调用 ID（tool_use 块）.
	// [EN] Tool call ID (tool_use blocks).
	ID string `json:"id,omitempty"`

	// Name 工具名（tool_use 块）.
	// [EN] Tool name (tool_use blocks).
	Name string `json:"name,omitempty"`

	// Input 工具参数对象（tool_use 块）.
	// [EN] Tool arguments object (tool_use blocks).
	Input any `json:"input,omitempty"`

	// ToolUseID 工具结果回传的目标调用 ID（tool_result 块）.
	// [EN] Target tool call ID (tool_result blocks).
	ToolUseID string `json:"tool_use_id,omitempty"`

	// Content 工具结果文本（tool_result 块）.
	// [EN] Tool result text (tool_result blocks).
	Content string `json:"content,omitempty"`

	// IsError 工具执行失败标记（tool_result 块）.
	// [EN] Tool failure flag (tool_result blocks).
	IsError bool `json:"is_error,omitempty"`
}

// wireImageSource 图片源（base64 内联或公网 URL）.
// [EN] Image source (inline base64 or public URL).
type wireImageSource struct {
	// Type 源类型（base64 / url）.
	// [EN] Source type (base64 / url).
	Type string `json:"type"`

	// MediaType MIME 类型（base64 源）.
	// [EN] MIME type (base64 source).
	MediaType string `json:"media_type,omitempty"`

	// Data base64 数据（base64 源）.
	// [EN] base64 payload (base64 source).
	Data string `json:"data,omitempty"`

	// URL 公网地址（url 源）.
	// [EN] Public URL (url source).
	URL string `json:"url,omitempty"`
}

// wireTool 工具定义.
// [EN] Tool definition.
type wireTool struct {
	// Name 工具名.
	// [EN] Tool name.
	Name string `json:"name"`

	// Description 功能描述.
	// [EN] Description.
	Description string `json:"description"`

	// InputSchema 参数 JSON Schema.
	// [EN] Arguments JSON Schema.
	InputSchema any `json:"input_schema"`
}

// ============================================================================
// 响应结构
// ============================================================================

// wireResponse Messages 响应体.
// [EN] Messages response body.
type wireResponse struct {
	// ID 消息 ID.
	// [EN] Message ID.
	ID string `json:"id"`

	// Model 实际模型.
	// [EN] Actual model.
	Model string `json:"model"`

	// Content 内容块列表.
	// [EN] Content blocks.
	Content []wireBlock `json:"content"`

	// StopReason 结束原因（end_turn/tool_use/max_tokens/stop_sequence）.
	// [EN] Stop reason.
	StopReason string `json:"stop_reason"`

	// Usage token 用量.
	// [EN] Token usage.
	Usage wireUsage `json:"usage"`

	// Type 顶层类型（错误响应体为 error）.
	// [EN] Top-level type (error bodies carry "error").
	Type string `json:"type,omitempty"`

	// Error 错误详情（错误响应体）.
	// [EN] Error detail (error bodies).
	Error *wireErrorBody `json:"error,omitempty"`
}

// wireUsage token 用量.
// [EN] Token usage.
type wireUsage struct {
	// InputTokens 输入 token.
	// [EN] Input tokens.
	InputTokens int `json:"input_tokens"`

	// OutputTokens 输出 token.
	// [EN] Output tokens.
	OutputTokens int `json:"output_tokens"`
}

// ============================================================================
// 编解码（llmx ↔ wire）
// ============================================================================

// encodeMessages llmx 消息 → wire 消息 + system 参数.
// [EN] Encode llmx messages to wire messages plus the system prompt.
//
// 协议差异：system 消息抽出为返回值；tool 角色编码为 user 消息内的 tool_result 块；
// 纯文本快速路径：单 TextPart 且非工具结果时 Content 直接发字符串
func encodeMessages(messages []llmx.Message) (msgs []wireMessage, system string) {
	var systems []string
	msgs = make([]wireMessage, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case llmx.RoleSystem:
			for _, p := range m.Content {
				if t, ok := p.(llmx.TextPart); ok {
					systems = append(systems, t.Text)
				}
			}
			continue
		case llmx.RoleTool:
			// 工具结果：user 消息 + tool_result 块
			msgs = append(msgs, wireMessage{Role: roleUser, Content: encodeToolResultBlocks(m)})
			continue
		}

		wm := wireMessage{Role: encodeRole(m.Role)}
		if len(m.Content) == 1 {
			if t, ok := m.Content[0].(llmx.TextPart); ok {
				wm.Content = t.Text
				msgs = append(msgs, wm)
				continue
			}
		}
		var blocks []wireBlock
		for _, p := range m.Content {
			switch part := p.(type) {
			case llmx.TextPart:
				blocks = append(blocks, wireBlock{Type: blockTypeText, Text: part.Text})
			case llmx.ImagePart:
				blocks = append(blocks, wireBlock{Type: blockTypeImage, Source: encodeImageSource(part)})
			case llmx.ToolCallPart:
				blocks = append(blocks, wireBlock{
					Type:  blockTypeToolUse,
					ID:    part.ID,
					Name:  part.Name,
					Input: parseArguments(part.Arguments),
				})
			}
		}
		if len(blocks) > 0 {
			wm.Content = blocks
		}
		msgs = append(msgs, wm)
	}
	return msgs, strings.Join(systems, "\n\n")
}

// encodeToolResult 工具结果消息 → wire tool_result 块.
// [EN] Encode a tool result message to wire tool_result blocks.
func encodeToolResultBlocks(m llmx.Message) []wireBlock {
	var blocks []wireBlock
	for _, p := range m.Content {
		if part, ok := p.(llmx.ToolResultPart); ok {
			blocks = append(blocks, wireBlock{
				Type:      blockTypeToolResult,
				ToolUseID: m.ToolCallID,
				Content:   part.Result,
				IsError:   part.Error != "",
			})
		}
	}
	return blocks
}

// encodeRole llmx Role → wire 字面量（system/tool 已在上游分流）.
// [EN] Map llmx Role to the wire literal (system/tool handled upstream).
func encodeRole(r llmx.Role) string {
	if r == llmx.RoleAssistant {
		return roleAssistant
	}
	return roleUser
}

// encodeImageSource 图片 Part → base64 内联源或 URL 源.
// [EN] Encode an image part to a base64 inline source or a URL source.
func encodeImageSource(p llmx.ImagePart) *wireImageSource {
	if p.URL != "" {
		return &wireImageSource{Type: sourceTypeURL, URL: p.URL}
	}
	mime := p.MIMEType
	if mime == "" {
		mime = defaultImageMIME
	}
	return &wireImageSource{Type: sourceTypeBase64, MediaType: mime, Data: base64.StdEncoding.EncodeToString(p.Data)}
}

// parseArguments 参数 JSON 串 → 对象（空/非法兜底空对象，协议要求 Input 为 JSON 对象）.
// [EN] Parse an arguments JSON string to an object (empty/invalid falls back).
func parseArguments(args string) any {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return map[string]any{}
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return map[string]any{}
	}
	return obj
}

// decodeChoice wire 响应 → llmx Choice.
// [EN] Decode the wire response to an llmx Choice.
func decodeChoice(wr *wireResponse) llmx.Choice {
	choice := llmx.Choice{FinishReason: wr.StopReason}
	var reasoning []string
	for _, b := range wr.Content {
		switch b.Type {
		case blockTypeText:
			choice.Content = append(choice.Content, llmx.TextPart{Text: b.Text})
		case blockTypeThinking:
			reasoning = append(reasoning, b.Thinking)
		case blockTypeToolUse:
			choice.Content = append(choice.Content, llmx.ToolCallPart{
				ID:        b.ID,
				Name:      b.Name,
				Arguments: encodeArguments(b.Input),
			})
		}
	}
	choice.Reasoning = strings.Join(reasoning, "")
	return choice
}

// encodeArguments 工具参数对象 → JSON 串（nil 兜底空对象，对齐 llmx ToolCallPart 语义）.
// [EN] Encode a tool arguments object to a JSON string (nil falls back).
func encodeArguments(input any) string {
	if input == nil {
		return emptyJSONObject
	}
	data, err := json.Marshal(input)
	if err != nil || string(data) == "null" {
		return emptyJSONObject
	}
	return string(data)
}
