/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 20:16:09
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 20:25:31
 * @FilePath: \go-llmx\adapters\mistral\wire.go
 * @Description: Mistral 协议编解码 —— wire 请求/响应结构与 llmx 类型互转.
 * Chat Completions 兼容形态；差异点：无 seed/reasoning、工具结果字符串回传
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmistral

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

	// Stream 流式开关.
	// [EN] Streaming flag.
	Stream bool `json:"stream,omitempty"`

	// Tools 工具定义.
	// [EN] Tool definitions.
	Tools []wireTool `json:"tools,omitempty"`

	// ResponseFormat 强制输出格式.
	// [EN] Forced response format.
	ResponseFormat *wireResponseFormat `json:"response_format,omitempty"`

	// User 终端用户标识.
	// [EN] End-user identifier.
	User string `json:"user,omitempty"`
}

// wireMessage 协议消息.
// [EN] Protocol message.
type wireMessage struct {
	// Role 角色.
	// [EN] Role.
	Role string `json:"role"`

	// Content 内容（string 或多模态 parts）.
	// [EN] Content.
	Content any `json:"content,omitempty"`

	// ToolCalls assistant 发起的工具调用.
	// [EN] Tool calls issued by the assistant.
	ToolCalls []wireToolUse `json:"tool_calls,omitempty"`

	// ToolCallID 工具结果回传的目标调用 ID.
	// [EN] Target tool call ID.
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Name 工具名（tool 消息可选）.
	// [EN] Tool name.
	Name string `json:"name,omitempty"`
}

// wireContentPart 多模态内容片段.
// [EN] Multimodal content part.
type wireContentPart struct {
	// Type 片段类型（text / image_url）.
	// [EN] Part type.
	Type string `json:"type"`

	// Text 文本内容.
	// [EN] Text content.
	Text string `json:"text,omitempty"`

	// ImageURL 图片引用.
	// [EN] Image reference.
	ImageURL *wireImageRef `json:"image_url,omitempty"`
}

// wireImageRef 图片引用（URL 或 base64 data URL）.
// [EN] Image reference.
type wireImageRef struct {
	// URL 图片地址.
	// [EN] Image URL.
	URL string `json:"url"`
}

// wireTool 工具定义容器.
// [EN] Tool definition container.
type wireTool struct {
	// Type 工具类型（function）.
	// [EN] Tool type.
	Type string `json:"type"`

	// Function 函数定义.
	// [EN] Function definition.
	Function wireFunction `json:"function"`
}

// wireFunction 函数定义.
// [EN] Function definition.
type wireFunction struct {
	// Name 函数名.
	// [EN] Function name.
	Name string `json:"name"`

	// Description 功能描述.
	// [EN] Description.
	Description string `json:"description"`

	// Parameters 参数 JSON Schema.
	// [EN] Parameter JSON Schema.
	Parameters any `json:"parameters,omitempty"`
}

// wireToolUse assistant 发起的工具调用.
// [EN] A tool call issued by the assistant.
type wireToolUse struct {
	// ID 调用 ID.
	// [EN] Call ID.
	ID string `json:"id"`

	// Type 类型（function）.
	// [EN] Type.
	Type string `json:"type"`

	// Function 调用载荷.
	// [EN] Call payload.
	Function wireFunctionCall `json:"function"`
}

// wireFunctionCall 调用载荷.
// [EN] Call payload.
type wireFunctionCall struct {
	// Name 函数名.
	// [EN] Function name.
	Name string `json:"name"`

	// Arguments JSON 编码的参数串.
	// [EN] JSON-encoded arguments.
	Arguments string `json:"arguments"`
}

// wireResponseFormat 强制输出格式.
// [EN] Forced response format.
type wireResponseFormat struct {
	// Type 格式类型（json_object）.
	// [EN] Format type.
	Type string `json:"type"`
}

// ============================================================================
// 响应结构
// ============================================================================

// wireResponse Chat Completions 响应体.
// [EN] Chat Completions response body.
type wireResponse struct {
	// Choices 候选列表.
	// [EN] Choices.
	Choices []wireChoice `json:"choices"`

	// Usage 用量统计.
	// [EN] Usage.
	Usage wireUsage `json:"usage"`

	// Model 实际模型名.
	// [EN] Actual model name.
	Model string `json:"model"`
}

// wireChoice 单个候选.
// [EN] A single choice.
type wireChoice struct {
	// Index 候选序号.
	// [EN] Choice index.
	Index int `json:"index"`

	// Message 回答消息.
	// [EN] Answer message.
	Message wireChoiceMessage `json:"message"`

	// FinishReason 结束原因（stop/length/tool_calls）.
	// [EN] Finish reason.
	FinishReason string `json:"finish_reason"`
}

// wireChoiceMessage 候选消息.
// [EN] Choice message.
type wireChoiceMessage struct {
	// Role 角色（assistant）.
	// [EN] Role.
	Role string `json:"role"`

	// Content 回答文本.
	// [EN] Answer text.
	Content string `json:"content"`

	// ToolCalls 工具调用请求.
	// [EN] Tool call requests.
	ToolCalls []wireToolUse `json:"tool_calls,omitempty"`
}

// wireUsage 用量.
// [EN] Usage.
type wireUsage struct {
	// PromptTokens 输入 token 数.
	// [EN] Prompt tokens.
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens 输出 token 数.
	// [EN] Completion tokens.
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens 总数.
	// [EN] Total tokens.
	TotalTokens int `json:"total_tokens"`
}

// ============================================================================
// 流式结构（SSE 增量帧）
// ============================================================================

// wireStreamChunk 流式增量帧.
// [EN] A streaming delta frame.
type wireStreamChunk struct {
	// Choices 候选增量.
	// [EN] Choice deltas.
	Choices []wireStreamChoice `json:"choices"`

	// Usage 用量（stream_options include_usage 时末帧携带）.
	// [EN] Usage (last frame with stream_options).
	Usage *wireUsage `json:"usage,omitempty"`
}

// wireStreamChoice 流式候选增量.
// [EN] A streaming choice delta.
type wireStreamChoice struct {
	// Index 候选序号.
	// [EN] Choice index.
	Index int `json:"index"`

	// Delta 增量载荷.
	// [EN] Delta payload.
	Delta wireStreamDelta `json:"delta"`

	// FinishReason 结束原因（末帧非空）.
	// [EN] Finish reason (non-empty on the last frame).
	FinishReason string `json:"finish_reason,omitempty"`
}

// wireStreamDelta 增量载荷.
// [EN] Delta payload.
type wireStreamDelta struct {
	// Role 角色（首帧 assistant）.
	// [EN] Role (first frame).
	Role string `json:"role,omitempty"`

	// Content 文本增量.
	// [EN] Text delta.
	Content string `json:"content,omitempty"`

	// ToolCalls 工具调用增量.
	// [EN] Tool call deltas.
	ToolCalls []wireToolCallDelta `json:"tool_calls,omitempty"`
}

// wireToolCallDelta 工具调用流式增量.
// [EN] A tool call delta.
type wireToolCallDelta struct {
	// Index 调用序号.
	// [EN] Call index.
	Index int `json:"index"`

	// ID 调用 ID（首帧）.
	// [EN] Call ID (first frame).
	ID string `json:"id,omitempty"`

	// Type 类型（function）.
	// [EN] Type.
	Type string `json:"type,omitempty"`

	// Function 函数载荷增量.
	// [EN] Function payload delta.
	Function wireFunctionCall `json:"function,omitempty"`
}

// ============================================================================
// 编解码（llmx.Message/Response <-> wire）
// ============================================================================

// buildRequest 组装请求体.
// [EN] Build the request body.
func buildRequest(o *llmx.Options, messages []llmx.Message, stream bool) *wireRequest {
	req := &wireRequest{
		Model:       o.Model,
		Temperature: o.Temperature,
		MaxTokens:   o.MaxTokens,
		TopP:        o.TopP,
		Stop:        o.Stop,
		N:           o.N,
		Stream:      stream,
		User:        o.User,
	}
	if len(req.Stop) == 0 {
		req.Stop = nil
	}
	if o.JSONMode {
		req.ResponseFormat = &wireResponseFormat{Type: responseFormatJSONObject}
	}
	for _, m := range messages {
		req.Messages = append(req.Messages, encodeMessage(m))
	}
	for _, d := range o.Tools {
		req.Tools = append(req.Tools, wireTool{Type: toolTypeFunction, Function: wireFunction{
			Name: d.Name, Description: d.Description, Parameters: d.Parameters,
		}})
	}
	return req
}

// encodeMessage llmx 消息 → wire 消息.
// [EN] Convert an llmx message to a wire message.
func encodeMessage(m llmx.Message) wireMessage {
	wm := wireMessage{Role: string(m.Role)}
	switch m.Role {
	case llmx.RoleAssistant:
		calls := m.ToolCalls()
		if len(calls) > 0 {
			for _, c := range calls {
				args := c.Arguments
				if args == "" {
					args = emptyJSONObject
				}
				wm.ToolCalls = append(wm.ToolCalls, wireToolUse{ID: c.ID, Type: toolTypeFunction, Function: wireFunctionCall{Name: c.Name, Arguments: args}})
			}
		}
		wm.Content = m.String()
	case llmx.RoleUser:
		if len(m.Content) == 1 {
			if t, ok := m.Content[0].(llmx.TextPart); ok {
				wm.Content = t.Text
			}
		} else {
			wm.Content = encodeParts(m)
		}
	case llmx.RoleTool:
		wm.ToolCallID = m.ToolCallID
		wm.Name = m.Name
		wm.Content = toolResultText(m)
	default:
		wm.Content = m.String()
	}
	return wm
}

// encodeParts 多模态消息 → 片段列表.
// [EN] Convert a multimodal message to parts.
func encodeParts(m llmx.Message) []wireContentPart {
	var parts []wireContentPart
	for _, p := range m.Content {
		switch v := p.(type) {
		case llmx.TextPart:
			parts = append(parts, wireContentPart{Type: partTypeText, Text: v.Text})
		case llmx.ImagePart:
			url := v.URL
			if len(v.Data) > 0 {
				mime := v.MIMEType
				if mime == "" {
					mime = "image/png"
				}
				url = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(v.Data)
			}
			if url != "" {
				parts = append(parts, wireContentPart{Type: "image_url", ImageURL: &wireImageRef{URL: url}})
			}
		}
	}
	return parts
}

// toolResultText 提取工具结果文本（错误前缀注入）.
// [EN] Extract the tool result text.
func toolResultText(m llmx.Message) string {
	var b strings.Builder
	for _, p := range m.Content {
		if tr, ok := p.(llmx.ToolResultPart); ok {
			if tr.Error != "" {
				b.WriteString("ERROR: ")
				b.WriteString(tr.Error)
			} else {
				b.WriteString(tr.Result)
			}
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// decodeResponse wire 响应 → llmx.Response.
// [EN] Convert a wire response to llmx.Response.
func decodeResponse(wr *wireResponse, model string) *llmx.Response {
	resp := &llmx.Response{
		Model: model,
		Usage: llmx.Usage{
			PromptTokens:     wr.Usage.PromptTokens,
			CompletionTokens: wr.Usage.CompletionTokens,
			TotalTokens:      wr.Usage.TotalTokens,
		},
	}
	if wr.Model != "" {
		resp.Model = wr.Model
	}
	for _, ch := range wr.Choices {
		c := llmx.Choice{FinishReason: ch.FinishReason}
		if ch.Message.Content != "" {
			c.Content = []llmx.Part{llmx.TextPart{Text: ch.Message.Content}}
		}
		for _, tc := range ch.Message.ToolCalls {
			args := tc.Function.Arguments
			if args == "" {
				args = emptyJSONObject
			}
			c.Content = append(c.Content, llmx.ToolCallPart{ID: tc.ID, Name: tc.Function.Name, Arguments: args})
		}
		resp.Choices = append(resp.Choices, c)
	}
	return resp
}
