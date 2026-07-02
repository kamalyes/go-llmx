/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 22:19:06
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 22:47:11
 * @FilePath: \go-llmx\adapters\cohere\wire.go
 * @Description: Cohere v2 协议编解码 —— wire 请求/响应结构与 llmx 类型互转.
 * tool 消息为双重结构：tool_calls 引用回传 + tool_result 内容项；
 * TopP 映射 p、无 TopK 暴露（固定协议缺省）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccohere

import (
	"encoding/base64"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// ============================================================================
// 请求结构
// ============================================================================

// wireRequest v2 chat 请求体.
// [EN] v2 chat request body.
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

	// P 核采样阈值（TopP 映射）.
	// [EN] Nucleus sampling (TopP mapped).
	P float64 `json:"p,omitempty"`

	// StopSequences 停止序列.
	// [EN] Stop sequences.
	StopSequences []string `json:"stop_sequences,omitempty"`

	// Seed 随机种子.
	// [EN] Random seed.
	Seed int `json:"seed,omitempty"`

	// Stream 流式开关.
	// [EN] Streaming flag.
	Stream bool `json:"stream,omitempty"`

	// Tools 工具定义.
	// [EN] Tool definitions.
	Tools []wireTool `json:"tools,omitempty"`

	// ResponseFormat 强制输出格式.
	// [EN] Forced response format.
	ResponseFormat *wireResponseFormat `json:"response_format,omitempty"`
}

// wireMessage 协议消息（content 为 string 或片段数组；tool 消息双重结构）.
// [EN] Protocol message (string or parts; tool messages carry the dual shape).
type wireMessage struct {
	// Role 角色.
	// [EN] Role.
	Role string `json:"role"`

	// Content 内容（system/user/assistant 为 string 或 []wireContentPart；tool 为 []wireToolResult）.
	// [EN] Content.
	Content any `json:"content"`

	// ToolCalls assistant 发起的工具调用（assistant/tool 消息携带，tool 消息回传原引用）.
	// [EN] Tool calls (in assistant; tool messages echo the original reference).
	ToolCalls []wireToolUse `json:"tool_calls,omitempty"`
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

// wireImageRef 图片引用.
// [EN] Image reference.
type wireImageRef struct {
	// URL 图片地址.
	// [EN] Image URL.
	URL string `json:"url"`
}

// wireToolResult 工具结果项（tool 消息 content 形态）.
// [EN] Tool result item (content shape of tool messages).
type wireToolResult struct {
	// Type 结果类型（tool_result）.
	// [EN] Result type.
	Type string `json:"type"`

	// ToolCallID 对应调用 ID.
	// [EN] Target call ID.
	ToolCallID string `json:"tool_call_id"`

	// Content 结果内容（文本片段列表）.
	// [EN] Result content.
	Content []wireContentPart `json:"content"`
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
	Parameters any `json:"parameters"`
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

// wireResponse v2 chat 响应体.
// [EN] v2 chat response body.
type wireResponse struct {
	// ID 响应 ID.
	// [EN] Response ID.
	ID string `json:"id"`

	// Message 回答消息.
	// [EN] Answer message.
	Message wireResponseMessage `json:"message"`

	// FinishReason 结束原因.
	// [EN] Finish reason.
	FinishReason string `json:"finish_reason"`

	// Usage 用量统计.
	// [EN] Usage.
	Usage wireUsage `json:"usage"`
}

// wireResponseMessage 回答消息.
// [EN] Answer message.
type wireResponseMessage struct {
	// Role 角色（assistant）.
	// [EN] Role.
	Role string `json:"role"`

	// Content 回答内容片段（text 类型）.
	// [EN] Answer content parts.
	Content []wireContentPart `json:"content"`

	// ToolCalls 工具调用请求.
	// [EN] Tool call requests.
	ToolCalls []wireToolUse `json:"tool_calls,omitempty"`
}

// wireUsage 用量.
// [EN] Usage.
type wireUsage struct {
	// Tokens 精确 token 计数.
	// [EN] Exact token counts.
	Tokens wireTokenCounts `json:"tokens"`
}

// wireTokenCounts token 计数.
// [EN] Token counts.
type wireTokenCounts struct {
	// InputTokens 输入.
	// [EN] Input.
	InputTokens int `json:"input_tokens"`

	// OutputTokens 输出.
	// [EN] Output.
	OutputTokens int `json:"output_tokens"`
}

// ============================================================================
// 编解码（llmx.Message/Response <-> wire）
// ============================================================================

// buildRequest 组装请求体.
// [EN] Build the request body.
func buildRequest(o *llmx.Options, messages []llmx.Message, stream bool) *wireRequest {
	req := &wireRequest{
		Temperature:   o.Temperature,
		MaxTokens:     o.MaxTokens,
		P:             o.TopP,
		StopSequences: o.Stop,
		Seed:          o.Seed,
		Stream:        stream,
	}
	if len(req.StopSequences) == 0 {
		req.StopSequences = nil
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
		if s := m.String(); s != "" || len(wm.ToolCalls) == 0 {
			wm.Content = s
		}
	case llmx.RoleTool:
		results := make([]wireToolResult, 0, len(m.Content))
		for _, p := range m.Content {
			if tr, ok := p.(llmx.ToolResultPart); ok {
				text := tr.Result
				if tr.Error != "" {
					text = toolResultErrorPrefix + tr.Error
				}
				results = append(results, wireToolResult{
					Type:       partTypeToolResult,
					ToolCallID: m.ToolCallID,
					Content:    []wireContentPart{{Type: partTypeText, Text: text}},
				})
			}
		}
		wm.Content = results
	case llmx.RoleUser:
		if len(m.Content) == 1 {
			if t, ok := m.Content[0].(llmx.TextPart); ok {
				wm.Content = t.Text
				break
			}
		}
		wm.Content = encodeParts(m)
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
				parts = append(parts, wireContentPart{Type: partTypeImageURL, ImageURL: &wireImageRef{URL: url}})
			}
		}
	}
	return parts
}

// decodeResponse wire 响应 → llmx.Response.
// [EN] Convert a wire response to llmx.Response.
func decodeResponse(wr *wireResponse, model string) *llmx.Response {
	resp := &llmx.Response{
		Model: model,
		Usage: llmx.Usage{
			PromptTokens:     wr.Usage.Tokens.InputTokens,
			CompletionTokens: wr.Usage.Tokens.OutputTokens,
			TotalTokens:      wr.Usage.Tokens.InputTokens + wr.Usage.Tokens.OutputTokens,
		},
	}
	ch := llmx.Choice{FinishReason: mapFinishReason(wr.FinishReason)}
	for _, p := range wr.Message.Content {
		if p.Text != "" {
			ch.Content = append(ch.Content, llmx.TextPart{Text: p.Text})
		}
	}
	for _, tc := range wr.Message.ToolCalls {
		args := tc.Function.Arguments
		if args == "" {
			args = emptyJSONObject
		}
		ch.Content = append(ch.Content, llmx.ToolCallPart{ID: tc.ID, Name: tc.Function.Name, Arguments: args})
	}
	if len(ch.Content) > 0 {
		resp.Choices = []llmx.Choice{ch}
	}
	return resp
}

// mapFinishReason Cohere 结束原因 → llmx 语义值.
// [EN] Map a Cohere finish reason to the llmx semantic value.
func mapFinishReason(reason string) string {
	switch reason {
	case finishComplete, finishStopSequence:
		return "stop"
	case finishMaxTokens:
		return "length"
	case finishToolCalls:
		return "tool_calls"
	default:
		return reason
	}
}

// joinText 拼接片段文本（流式聚合辅助）.
// [EN] Join part texts (streaming helper).
func joinText(parts []wireContentPart) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
	}
	return b.String()
}
