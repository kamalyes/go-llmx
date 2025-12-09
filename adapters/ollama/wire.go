/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 20:58:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 20:58:00
 * @FilePath: \go-llmx\adapters\ollama\wire.go
 * @Description: Ollama 协议编解码 —— wire 请求/响应结构与 llmx 类型互转.
 * /api/chat 协议：content 恒为纯文本串、images 为 base64 数组、工具参数为 JSON
 * 对象（非字符串）、采样参数收拢在 options、无调用 ID（合成）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollama

import (
	"encoding/base64"
	"encoding/json"

	llmx "github.com/kamalyes/go-llmx"
)

// ============================================================================
// 请求结构
// ============================================================================

// wireRequest /api/chat 请求体.
// [EN] /api/chat request body.
type wireRequest struct {
	// Model 模型名.
	// [EN] Model name.
	Model string `json:"model"`

	// Messages 对话消息列表.
	// [EN] Conversation messages.
	Messages []wireMessage `json:"messages"`

	// Stream 流式开关.
	// [EN] Streaming flag.
	Stream bool `json:"stream"`

	// Tools 工具定义.
	// [EN] Tool definitions.
	Tools []wireTool `json:"tools,omitempty"`

	// Format 强制输出格式（json）.
	// [EN] Forced output format (json).
	Format string `json:"format,omitempty"`

	// Options 采样参数容器（协议将 OpenAI 风格参数收拢于此）.
	// [EN] Sampling options container.
	Options *wireOptions `json:"options,omitempty"`
}

// wireMessage 协议消息（content 恒为纯文本串）.
// [EN] Protocol message (content is always a plain string).
type wireMessage struct {
	// Role 角色（system/user/assistant/tool）.
	// [EN] Role.
	Role string `json:"role"`

	// Content 文本内容.
	// [EN] Text content.
	Content string `json:"content"`

	// Images 内联图片 base64 数组（多模态模型消费）.
	// [EN] Inline base64 images.
	Images []string `json:"images,omitempty"`

	// ToolCalls assistant 发起的工具调用（参数为 JSON 对象）.
	// [EN] Tool calls issued by the assistant (arguments as JSON object).
	ToolCalls []wireToolCall `json:"tool_calls,omitempty"`
}

// wireToolCall 工具调用（协议无调用 ID）.
// [EN] A tool call (protocol carries no call ID).
type wireToolCall struct {
	// Function 函数名与参数.
	// [EN] Function name and arguments.
	Function wireFunctionCall `json:"function"`
}

// wireFunctionCall 函数调用载荷（Arguments 为 JSON 对象）.
// [EN] Function call payload (Arguments is a JSON object).
type wireFunctionCall struct {
	// Name 函数名.
	// [EN] Function name.
	Name string `json:"name"`

	// Arguments 参数对象（非字符串，与 OpenAI 协议差异点）.
	// [EN] Argument object (not a string, unlike OpenAI).
	Arguments any `json:"arguments"`
}

// wireOptions 采样参数（llmx Options 的协议映射层）.
// [EN] Sampling options (protocol mapping of llmx Options).
type wireOptions struct {
	// Temperature 采样温度.
	// [EN] Sampling temperature.
	Temperature float64 `json:"temperature,omitempty"`

	// TopP 核采样阈值.
	// [EN] Nucleus sampling threshold.
	TopP float64 `json:"top_p,omitempty"`

	// NumPredict 输出 token 上限（max_tokens 的协议名）.
	// [EN] Max output tokens (protocol name of max_tokens).
	NumPredict int `json:"num_predict,omitempty"`

	// Seed 随机种子.
	// [EN] Random seed.
	Seed int `json:"seed,omitempty"`

	// Stop 停止序列.
	// [EN] Stop sequences.
	Stop []string `json:"stop,omitempty"`
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
}

// ============================================================================
// 响应结构（非流式与流式单帧同构）
// ============================================================================

// wireResponse /api/chat 响应体（流式时为单帧形态）.
// [EN] /api/chat response body (single frame when streaming).
type wireResponse struct {
	// Message 回复消息（非流式完整 / 流式增量）.
	// [EN] Reply message (full or delta).
	Message wireMessage `json:"message"`

	// Done 结束标记（流式末帧为 true）.
	// [EN] Done marker (true on the final streaming frame).
	Done bool `json:"done"`

	// DoneReason 结束原因（stop/length；done 帧携带）.
	// [EN] Finish reason (carried on the done frame).
	DoneReason string `json:"done_reason"`

	// PromptEvalCount 输入 token 数（done 帧携带）.
	// [EN] Prompt token count (on the done frame).
	PromptEvalCount int `json:"prompt_eval_count"`

	// EvalCount 输出 token 数（done 帧携带）.
	// [EN] Completion token count (on the done frame).
	EvalCount int `json:"eval_count"`

	// Error 错误信息（任意状态均可能注入）.
	// [EN] Error message (injected on any status).
	Error string `json:"error,omitempty"`
}

// ============================================================================
// 编解码（llmx ↔ wire）
// ============================================================================

// encodeMessages llmx 消息 → wire 协议.
// [EN] Encode llmx messages to the wire protocol.
//
// 多模态：文本 Part 拼接为 content 串、内联图片走 images（协议不支持 URL 图片，忽略）；
// 工具结果 RoleTool 编码为 role:tool 的文本消息；assistant 工具调用参数解析为对象
func encodeMessages(messages []llmx.Message) []wireMessage {
	out := make([]wireMessage, 0, len(messages))
	for _, m := range messages {
		wm := wireMessage{Role: encodeRole(m.Role)}
		for _, p := range m.Content {
			switch part := p.(type) {
			case llmx.TextPart:
				wm.Content += part.Text
			case llmx.ImagePart:
				if len(part.Data) > 0 {
					wm.Images = append(wm.Images, base64.StdEncoding.EncodeToString(part.Data))
				}
			case llmx.ToolResultPart:
				text := part.Result
				if part.Error != "" {
					text = toolResultErrorPrefix + part.Error
				}
				if wm.Content == "" {
					wm.Content = text
				} else {
					wm.Content += "\n" + text
				}
			case llmx.ToolCallPart:
				wm.ToolCalls = append(wm.ToolCalls, wireToolCall{
					Function: wireFunctionCall{
						Name:      part.Name,
						Arguments: parseArguments(part.Arguments),
					},
				})
			}
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

// parseArguments 参数 JSON 串 → 对象（空/非法兜底空对象，协议要求对象形态）.
// [EN] Parse an argument JSON string into an object (fallback empty object).
func parseArguments(s string) map[string]any {
	if s == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

// argumentsToJSON 参数对象 → JSON 串（编码失败兜底空对象）.
// [EN] Encode an argument object to a JSON string (fallback empty object).
func argumentsToJSON(v any) string {
	if v == nil {
		return emptyJSONObject
	}
	b, err := json.Marshal(v)
	if err != nil {
		return emptyJSONObject
	}
	return string(b)
}

// decodeChoice wire 响应 → llmx Choice.
// [EN] Decode a wire response to llmx Choice.
func decodeChoice(wr *wireResponse) llmx.Choice {
	choice := llmx.Choice{FinishReason: wr.DoneReason}
	if wr.Message.Content != "" {
		choice.Content = append(choice.Content, llmx.TextPart{Text: wr.Message.Content})
	}
	for _, tc := range wr.Message.ToolCalls {
		choice.Content = append(choice.Content, llmx.ToolCallPart{
			ID:        toolCallIDPrefix + tc.Function.Name,
			Name:      tc.Function.Name,
			Arguments: argumentsToJSON(tc.Function.Arguments),
		})
	}
	return choice
}

// usage wire 帧用量 → llmx Usage（协议无 total，本地求和）.
// [EN] Wire frame counts to llmx Usage (protocol has no total; summed locally).
func usage(wr *wireResponse) llmx.Usage {
	return llmx.Usage{
		PromptTokens:     wr.PromptEvalCount,
		CompletionTokens: wr.EvalCount,
		TotalTokens:      wr.PromptEvalCount + wr.EvalCount,
	}
}
