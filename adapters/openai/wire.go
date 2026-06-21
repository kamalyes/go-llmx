/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-01 21:01:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-21 20:58:02
 * @FilePath: \go-llmx\adapters\openai\wire.go
 * @Description: OpenAI 兼容协议编解码 —— wire 请求/响应结构与 llmx 类型互转.
 * 仅本包可见：适配器主体只编排调用，协议细节全部收口在此
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"encoding/base64"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// ============================================================================
// 请求结构
// ============================================================================

// wireRequest Chat Completions 请求体.
// [EN] Chat Completions request body.
type wireRequest struct {
	// Model 模型名.
	// [EN] Model name.
	Model string `json:"model"`

	// Messages 对话消息列表.
	// [EN] Conversation messages.
	Messages []wireMessage `json:"messages"`

	// Temperature 采样温度.
	// [EN] Sampling temperature.
	Temperature float64 `json:"temperature,omitempty"`

	// MaxTokens 输出 token 上限.
	// [EN] Max output tokens.
	MaxTokens int `json:"max_tokens,omitempty"`

	// TopP 核采样阈值.
	// [EN] Nucleus sampling threshold.
	TopP float64 `json:"top_p,omitempty"`

	// Stop 停止序列.
	// [EN] Stop sequences.
	Stop []string `json:"stop,omitempty"`

	// N 候选数.
	// [EN] Number of choices.
	N int `json:"n,omitempty"`

	// Seed 随机种子（指针区分未设置与 0）.
	// [EN] Random seed (pointer to distinguish unset from 0).
	Seed *int `json:"seed,omitempty"`

	// Stream 流式开关.
	// [EN] Streaming flag.
	Stream bool `json:"stream,omitempty"`

	// Tools 工具定义.
	// [EN] Tool definitions.
	Tools []wireTool `json:"tools,omitempty"`

	// ResponseFormat 强制输出格式.
	// [EN] Forced response format.
	ResponseFormat *wireResponseFormat `json:"response_format,omitempty"`

	// User 终端用户标识（风控/计费）.
	// [EN] End-user identifier (risk control / billing).
	User string `json:"user,omitempty"`

	// ReasoningEffort 思考强度（o 系模型；空串不携带）.
	// [EN] Reasoning effort (o-series models; empty omits it).
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// wireMessage 协议消息.
// [EN] Protocol message.
type wireMessage struct {
	// Role 角色（system/user/assistant/tool）.
	// [EN] Role.
	Role string `json:"role"`

	// Content 内容（string 纯文本 或 []wireContentPart 多模态）.
	// [EN] Content (string for plain text, or parts array for multimodal).
	Content any `json:"content,omitempty"`

	// ToolCalls assistant 发起的工具调用.
	// [EN] Tool calls issued by the assistant.
	ToolCalls []wireToolUse `json:"tool_calls,omitempty"`

	// ToolCallID 工具结果回传的目标调用 ID.
	// [EN] Target tool call ID for tool results.
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Name 工具名（tool 消息可选）.
	// [EN] Tool name (optional for tool messages).
	Name string `json:"name,omitempty"`

	// Reasoning 思考轨迹（DeepSeek reasoner 协议扩展）.
	// [EN] Reasoning trace (DeepSeek reasoner extension).
	Reasoning string `json:"reasoning_content,omitempty"`
}

// wireContentPart 多模态内容片段.
// [EN] Multimodal content part.
type wireContentPart struct {
	// Type 片段类型（text / image_url）.
	// [EN] Part type (text / image_url).
	Type string `json:"type"`

	// Text 文本内容（Type=text 时有效）.
	// [EN] Text content (when Type is text).
	Text string `json:"text,omitempty"`

	// ImageURL 图片引用（Type=image_url 时有效）.
	// [EN] Image reference (when Type is image_url).
	ImageURL *wireImageURL `json:"image_url,omitempty"`
}

// wireImageURL 图片引用.
// [EN] Image reference.
type wireImageURL struct {
	// URL 公网地址或 data:<mime>;base64,<data> 内联.
	// [EN] Public URL or inline data URL.
	URL string `json:"url"`
}

// wireTool 工具定义.
// [EN] Tool definition.
type wireTool struct {
	// Type 工具类型（固定 function）.
	// [EN] Tool type (always function).
	Type string `json:"type"`

	// Function 函数签名.
	// [EN] Function signature.
	Function wireToolSchema `json:"function"`
}

// wireToolSchema 函数签名.
// [EN] Function signature.
type wireToolSchema struct {
	// Name 工具名.
	// [EN] Tool name.
	Name string `json:"name"`

	// Description 功能描述.
	// [EN] Description.
	Description string `json:"description"`

	// Parameters 参数 JSON Schema.
	// [EN] Parameters JSON Schema.
	Parameters any `json:"parameters,omitempty"`

	// Strict 严格模式（Structured Outputs 保证参数合法）.
	// [EN] Strict mode (Structured Outputs guarantee).
	Strict bool `json:"strict,omitempty"`
}

// wireResponseFormat 强制输出格式.
// [EN] Forced response format.
type wireResponseFormat struct {
	// Type 格式类型（json_object）.
	// [EN] Format type (json_object).
	Type string `json:"type"`
}

// wireToolUse assistant 消息中的工具调用.
// [EN] Tool call in assistant messages.
type wireToolUse struct {
	// Index 流式增量中的聚合序号（仅响应侧使用）.
	// [EN] Aggregation index in streaming deltas (response side only).
	Index int `json:"index,omitempty"`

	// ID 调用 ID.
	// [EN] Call ID.
	ID string `json:"id"`

	// Type 固定 function.
	// [EN] Always function.
	Type string `json:"type"`

	// Function 函数名与参数.
	// [EN] Function name and arguments.
	Function wireFunctionCall `json:"function"`
}

// wireFunctionCall 函数调用载荷.
// [EN] Function call payload.
type wireFunctionCall struct {
	// Name 函数名.
	// [EN] Function name.
	Name string `json:"name"`

	// Arguments JSON 编码参数串.
	// [EN] JSON-encoded argument string.
	Arguments string `json:"arguments"`
}

// ============================================================================
// 响应结构
// ============================================================================

// wireResponse Chat Completions 响应体.
// [EN] Chat Completions response body.
type wireResponse struct {
	// Choices 候选列表.
	// [EN] Candidate list.
	Choices []wireChoice `json:"choices"`

	// Usage token 用量.
	// [EN] Token usage.
	Usage wireUsage `json:"usage"`

	// Error 200 状态下网关仍可能注入的错误字段.
	// [EN] Error field some gateways inject even on 200.
	Error *wireErrorBody `json:"error,omitempty"`
}

// wireChoice 单个候选.
// [EN] A single candidate.
type wireChoice struct {
	// Message 完整消息（非流式）.
	// [EN] Full message (non-streaming).
	Message wireMessage `json:"message"`

	// FinishReason 结束原因（stop/length/tool_calls/content_filter）.
	// [EN] Finish reason.
	FinishReason string `json:"finish_reason"`
}

// wireUsage token 用量.
// [EN] Token usage.
type wireUsage struct {
	// PromptTokens 输入 token.
	// [EN] Prompt tokens.
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens 输出 token.
	// [EN] Completion tokens.
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens 总量.
	// [EN] Total tokens.
	TotalTokens int `json:"total_tokens"`
}

// ============================================================================
// 编解码（llmx ↔ wire）
// ============================================================================

// encodeMessages llmx 消息 → wire 协议.
// [EN] Encode llmx messages to the wire protocol.
//
// 纯文本快速路径：单 TextPart 且非工具结果时 Content 直接发字符串（省 token、兼容性最好）
func encodeMessages(messages []llmx.Message) []wireMessage {
	out := make([]wireMessage, 0, len(messages))
	for _, m := range messages {
		wm := wireMessage{Role: encodeRole(m.Role), ToolCallID: m.ToolCallID, Name: m.Name}

		if m.ToolCallID == "" && len(m.Content) == 1 {
			if t, ok := m.Content[0].(llmx.TextPart); ok {
				wm.Content = t.Text
				out = append(out, wm)
				continue
			}
		}

		var parts []wireContentPart
		for _, p := range m.Content {
			switch part := p.(type) {
			case llmx.TextPart:
				parts = append(parts, wireContentPart{Type: partTypeText, Text: part.Text})
			case llmx.ImagePart:
				parts = append(parts, wireContentPart{Type: partTypeImageURL, ImageURL: &wireImageURL{URL: encodeImage(part)}})
			case llmx.ToolResultPart:
				text := part.Result
				if part.Error != "" {
					text = toolResultErrorPrefix + part.Error
				}
				parts = append(parts, wireContentPart{Type: partTypeText, Text: text})
			case llmx.ToolCallPart:
				wm.ToolCalls = append(wm.ToolCalls, wireToolUse{
					ID:       part.ID,
					Type:     toolTypeFunction,
					Function: wireFunctionCall{Name: part.Name, Arguments: orDefaultJSON(part.Arguments)},
				})
			}
		}
		if len(parts) > 0 {
			wm.Content = parts
		}
		out = append(out, wm)
	}
	return out
}

// encodeRole llmx Role → wire 字面量.
// [EN] Map llmx Role to the wire literal.
func encodeRole(r llmx.Role) string {
	switch r {
	case llmx.RoleSystem:
		return roleSystem
	case llmx.RoleAssistant:
		return roleAssistant
	case llmx.RoleTool:
		return roleTool
	default:
		return roleUser
	}
}

// encodeImage 图片 Part → data URL 或原样 URL.
// [EN] Encode an image part to a data URL or pass through.
func encodeImage(p llmx.ImagePart) string {
	if p.URL != "" {
		return p.URL
	}
	mime := p.MIMEType
	if mime == "" {
		mime = defaultImageMIME
	}
	return dataURLPrefix + mime + dataURLBase64Sep + base64.StdEncoding.EncodeToString(p.Data)
}

// orDefaultJSON 空参数兜底为空对象（协议要求 Arguments 为合法 JSON 串）.
// [EN] Default empty arguments to an empty JSON object.
func orDefaultJSON(s string) string {
	if strings.TrimSpace(s) == "" {
		return emptyJSONObject
	}
	return s
}

// decodeChoice wire 消息 → llmx Choice.
// [EN] Decode a wire message to llmx Choice.
func decodeChoice(ch wireChoice) llmx.Choice {
	choice := llmx.Choice{
		Reasoning:    ch.Message.Reasoning,
		FinishReason: ch.FinishReason,
	}
	choice.Content = decodeParts(ch.Message)
	return choice
}

// decodeParts wire 消息内容 → llmx Part 集合.
// [EN] Decode wire message content to llmx parts.
func decodeParts(m wireMessage) []llmx.Part {
	var parts []llmx.Part
	if s, ok := m.Content.(string); ok && s != "" {
		parts = append(parts, llmx.TextPart{Text: s})
	} else if arr, ok := m.Content.([]any); ok {
		for _, el := range arr {
			mp, ok := el.(map[string]any)
			if !ok {
				continue
			}
			if mp["type"] == partTypeText {
				if t, _ := mp["text"].(string); t != "" {
					parts = append(parts, llmx.TextPart{Text: t})
				}
			}
		}
	}
	for _, tc := range m.ToolCalls {
		parts = append(parts, llmx.ToolCallPart{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return parts
}
