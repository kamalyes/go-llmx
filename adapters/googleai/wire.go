/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-01 20:58:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-01 21:07:19
 * @FilePath: \go-llmx\adapters\googleai\wire.go
 * @Description: Gemini 协议编解码 —— wire 请求/响应结构与 llmx 类型互转.
 * 模型名拼入 URL 路径（/models/{model}:generateContent），不走 Base.Path
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgoogleai

import (
	"encoding/base64"
	"encoding/json"

	llmx "github.com/kamalyes/go-llmx"
)

// ============================================================================
// 请求结构
// ============================================================================

// wireRequest generateContent 请求体.
// [EN] generateContent request body.
type wireRequest struct {
	// Contents 对话条目（role: user/model）.
	// [EN] Conversation entries.
	Contents []wireContent `json:"contents"`

	// SystemInstruction 系统指令（v1beta 独立字段，多条 system 消息合并）.
	// [EN] System instruction (merged from system messages).
	SystemInstruction *wireContent `json:"systemInstruction,omitempty"`

	// Tools 工具声明.
	// [EN] Tool declarations.
	Tools []wireTool `json:"tools,omitempty"`

	// GenerationConfig 生成参数.
	// [EN] Generation parameters.
	GenerationConfig *wireGenerationConfig `json:"generationConfig,omitempty"`
}

// wireContent 对话条目（role + parts）.
// [EN] A conversation entry.
type wireContent struct {
	// Role 角色（user/model；functionResponse 归入 user 条目）.
	// [EN] Role.
	Role string `json:"role"`

	// Parts 内容片段（text/inlineData/functionCall/functionResponse）.
	// [EN] Content parts.
	Parts []wirePart `json:"parts"`
}

// wirePart 内容片段（四种形态字段并集）.
// [EN] A content part (field union of four shapes).
type wirePart struct {
	// Text 文本.
	// [EN] Text.
	Text string `json:"text,omitempty"`

	// InlineData 内联二进制（图片 base64）.
	// [EN] Inline binary (base64 image).
	InlineData *wireInlineData `json:"inlineData,omitempty"`

	// FileData 文件引用（File API 上传地址）.
	// [EN] File reference (File API URI).
	FileData *wireFileData `json:"fileData,omitempty"`

	// FunctionCall 模型发起的函数调用.
	// [EN] Function call issued by the model.
	FunctionCall *wireFunctionCall `json:"functionCall,omitempty"`

	// FunctionResponse 函数执行结果回传.
	// [EN] Function execution result.
	FunctionResponse *wireFunctionResponse `json:"functionResponse,omitempty"`
}

// wireInlineData 内联数据.
// [EN] Inline data.
type wireInlineData struct {
	// MimeType 数据类型.
	// [EN] Data type.
	MimeType string `json:"mimeType"`

	// Data base64 编码内容.
	// [EN] Base64-encoded content.
	Data string `json:"data"`
}

// wireFileData 文件引用.
// [EN] File reference.
type wireFileData struct {
	// FileURI 文件地址.
	// [EN] File URI.
	FileURI string `json:"fileUri"`
}

// wireFunctionCall 函数调用（args 为 JSON 对象，宽松任意类型）.
// [EN] Function call (args as a JSON object).
type wireFunctionCall struct {
	// Name 函数名.
	// [EN] Function name.
	Name string `json:"name"`

	// Args 调用参数.
	// [EN] Call arguments.
	Args json.RawMessage `json:"args,omitempty"`
}

// wireFunctionResponse 函数结果（协议要求 response 为 JSON 对象）.
// [EN] Function result (response must be a JSON object).
type wireFunctionResponse struct {
	// Name 函数名（与调用名对应，非调用 ID）.
	// [EN] Function name (matches the call, not its ID).
	Name string `json:"name"`

	// Response 结果对象（错误时 {"error": msg}）.
	// [EN] Result object ({"error": msg} on failure).
	Response map[string]any `json:"response"`
}

// wireTool 工具容器（functionDeclarations 列表形态）.
// [EN] Tool container.
type wireTool struct {
	// FunctionDeclarations 函数声明.
	// [EN] Function declarations.
	FunctionDeclarations []wireFunctionDeclaration `json:"functionDeclarations"`
}

// wireFunctionDeclaration 函数声明.
// [EN] Function declaration.
type wireFunctionDeclaration struct {
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

// wireGenerationConfig 生成参数.
// [EN] Generation parameters.
type wireGenerationConfig struct {
	// Temperature 采样温度.
	// [EN] Sampling temperature.
	Temperature float64 `json:"temperature,omitempty"`

	// TopP 核采样阈值.
	// [EN] Nucleus sampling threshold.
	TopP float64 `json:"topP,omitempty"`

	// MaxOutputTokens 输出 token 上限.
	// [EN] Max output tokens.
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`

	// CandidateCount 候选数.
	// [EN] Candidate count.
	CandidateCount int `json:"candidateCount,omitempty"`

	// StopSequences 停止序列.
	// [EN] Stop sequences.
	StopSequences []string `json:"stopSequences,omitempty"`

	// ResponseMimeType 强制输出 MIME（JSONMode 映射 application/json）.
	// [EN] Forced output MIME.
	ResponseMimeType string `json:"responseMimeType,omitempty"`

	// Seed 随机种子.
	// [EN] Random seed.
	Seed *int `json:"seed,omitempty"`

	// ThinkingConfig 思考配置（Gemini 2.5+ thinkingBudget）.
	// [EN] Thinking config.
	ThinkingConfig *wireThinkingConfig `json:"thinkingConfig,omitempty"`
}

// wireThinkingConfig 思考配置.
// [EN] Thinking config.
type wireThinkingConfig struct {
	// ThinkingBudget 思考 token 预算（0 关闭思考）.
	// [EN] Thinking token budget (0 disables).
	ThinkingBudget int `json:"thinkingBudget"`
}

// ============================================================================
// 响应结构
// ============================================================================

// wireResponse generateContent 响应体.
// [EN] generateContent response body.
type wireResponse struct {
	// Candidates 候选回答.
	// [EN] Candidate answers.
	Candidates []wireCandidate `json:"candidates"`

	// UsageMetadata 用量统计.
	// [EN] Usage metadata.
	UsageMetadata *wireUsageMetadata `json:"usageMetadata,omitempty"`

	// ModelVersion 实际模型版本.
	// [EN] Actual model version.
	ModelVersion string `json:"modelVersion,omitempty"`
}

// wireCandidate 单个候选.
// [EN] A single candidate.
type wireCandidate struct {
	// Content 回答内容.
	// [EN] Answer content.
	Content *wireContent `json:"content,omitempty"`

	// FinishReason 结束原因（STOP/MAX_TOKENS/SAFETY/RECITATION）.
	// [EN] Finish reason.
	FinishReason string `json:"finishReason,omitempty"`
}

// wireUsageMetadata 用量.
// [EN] Usage metadata.
type wireUsageMetadata struct {
	// PromptTokenCount 输入 token 数.
	// [EN] Prompt token count.
	PromptTokenCount int `json:"promptTokenCount"`

	// CandidatesTokenCount 输出 token 数.
	// [EN] Output token count.
	CandidatesTokenCount int `json:"candidatesTokenCount"`

	// TotalTokenCount 总数.
	// [EN] Total count.
	TotalTokenCount int `json:"totalTokenCount"`
}

// ============================================================================
// 编解码（llmx.Message/Response <-> wire）
// ============================================================================

// buildRequest 组装请求体（system 合并进 SystemInstruction；tool 结果归入 user 条目）.
// [EN] Build the request body.
func buildRequest(o *llmx.Options, messages []llmx.Message) *wireRequest {
	req := &wireRequest{}
	var sysParts []wirePart

	for _, m := range messages {
		switch m.Role {
		case llmx.RoleSystem:
			for _, p := range m.Content {
				if t, ok := p.(llmx.TextPart); ok {
					sysParts = append(sysParts, wirePart{Text: t.Text})
				}
			}
		case llmx.RoleUser:
			req.Contents = append(req.Contents, wireContent{Role: roleUser, Parts: encodeParts(m)})
		case llmx.RoleAssistant:
			req.Contents = append(req.Contents, wireContent{Role: roleModel, Parts: encodeParts(m)})
		case llmx.RoleTool:
			// functionResponse 归入 user 条目（v1beta 协议形态）
			req.Contents = append(req.Contents, wireContent{Role: roleUser, Parts: encodeToolResult(m)})
		}
	}
	if len(sysParts) > 0 {
		req.SystemInstruction = &wireContent{Parts: sysParts}
	}

	if len(o.Tools) > 0 {
		decls := make([]wireFunctionDeclaration, len(o.Tools))
		for i, d := range o.Tools {
			decls[i] = wireFunctionDeclaration{Name: d.Name, Description: d.Description, Parameters: d.Parameters}
		}
		req.Tools = []wireTool{{FunctionDeclarations: decls}}
	}

	cfg := &wireGenerationConfig{
		Temperature:     o.Temperature,
		TopP:            o.TopP,
		MaxOutputTokens: o.MaxTokens,
		CandidateCount:  o.N,
		StopSequences:   o.Stop,
	}
	if len(cfg.StopSequences) == 0 {
		cfg.StopSequences = nil
	}
	if o.Seed != 0 {
		s := o.Seed
		cfg.Seed = &s
	}
	if o.JSONMode {
		cfg.ResponseMimeType = mimeJSON
	}
	if th := o.Thinking; th != nil {
		budget := th.BudgetTokens
		if budget <= 0 {
			budget = llmx.CalculateThinkingBudget(th.Mode, o.MaxTokens)
		}
		cfg.ThinkingConfig = &wireThinkingConfig{ThinkingBudget: budget}
	}
	req.GenerationConfig = cfg
	return req
}

// encodeParts 消息 Part 列表 → wire 片段.
// [EN] Convert message parts to wire parts.
func encodeParts(m llmx.Message) []wirePart {
	var parts []wirePart
	for _, p := range m.Content {
		switch v := p.(type) {
		case llmx.TextPart:
			parts = append(parts, wirePart{Text: v.Text})
		case llmx.ImagePart:
			if len(v.Data) > 0 {
				mime := v.MIMEType
				if mime == "" {
					mime = defaultImageMIME
				}
				parts = append(parts, wirePart{InlineData: &wireInlineData{
					MimeType: mime,
					Data:     base64.StdEncoding.EncodeToString(v.Data),
				}})
			} else if v.URL != "" {
				parts = append(parts, wirePart{FileData: &wireFileData{FileURI: v.URL}})
			}
		case llmx.ToolCallPart:
			args := json.RawMessage(defaultArgs(v.Arguments))
			parts = append(parts, wirePart{FunctionCall: &wireFunctionCall{Name: v.Name, Args: args}})
		}
	}
	return parts
}

// encodeToolResult 工具结果消息 → functionResponse 片段.
// [EN] Convert a tool result message to functionResponse parts.
func encodeToolResult(m llmx.Message) []wirePart {
	resp := map[string]any{}
	for _, p := range m.Content {
		if tr, ok := p.(llmx.ToolResultPart); ok {
			if tr.Error != "" {
				resp[toolErrorKey] = tr.Error
			} else {
				resp[resultKey] = tr.Result
			}
		}
	}
	name := m.Name
	if name == "" {
		name = m.ToolCallID
	}
	return []wirePart{{FunctionResponse: &wireFunctionResponse{Name: name, Response: resp}}}
}

// decodeResponse wire 响应 → llmx.Response.
// [EN] Convert a wire response to llmx.Response.
func decodeResponse(wr *wireResponse, model string) *llmx.Response {
	resp := &llmx.Response{Model: model}
	if wr.ModelVersion != "" {
		resp.Model = wr.ModelVersion
	}
	if wr.UsageMetadata != nil {
		resp.Usage = llmx.Usage{
			PromptTokens:     wr.UsageMetadata.PromptTokenCount,
			CompletionTokens: wr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      wr.UsageMetadata.TotalTokenCount,
		}
	}
	for _, cand := range wr.Candidates {
		ch := llmx.Choice{FinishReason: mapFinishReason(cand.FinishReason, cand.Content)}
		if cand.Content != nil {
			ch.Content = decodeParts(cand.Content.Parts)
		}
		resp.Choices = append(resp.Choices, ch)
	}
	return resp
}

// decodeParts wire 片段 → llmx Part 列表.
// [EN] Convert wire parts to llmx parts.
func decodeParts(parts []wirePart) []llmx.Part {
	var out []llmx.Part
	for _, p := range parts {
		switch {
		case p.Text != "":
			out = append(out, llmx.TextPart{Text: p.Text})
		case p.FunctionCall != nil:
			out = append(out, llmx.ToolCallPart{
				ID:        p.FunctionCall.Name, // Gemini 以函数名为调用标识（无独立 ID）
				Name:      p.FunctionCall.Name,
				Arguments: rawToJSONString(p.FunctionCall.Args),
			})
		case p.InlineData != nil:
			data, _ := base64.StdEncoding.DecodeString(p.InlineData.Data)
			out = append(out, llmx.ImagePart{MIMEType: p.InlineData.MimeType, Data: data})
		case p.FileData != nil:
			out = append(out, llmx.ImagePart{URL: p.FileData.FileURI})
		}
	}
	return out
}

// mapFinishReason Gemini 结束原因 → llmx 语义值.
// [EN] Map a Gemini finish reason to the llmx semantic value.
func mapFinishReason(reason string, content *wireContent) string {
	if content != nil {
		for _, p := range content.Parts {
			if p.FunctionCall != nil {
				return "tool_calls"
			}
		}
	}
	switch reason {
	case finishStop:
		return "stop"
	case finishMaxTokens:
		return "length"
	case finishSafety, finishRecitation:
		return "content_filter"
	default:
		return reason
	}
}

// defaultArgs 归一参数串（空串兜底 "{}"，保持合法 JSON 形态）.
// [EN] Normalize an argument string (empty falls back to "{}").
func defaultArgs(s string) string {
	if s == "" {
		return emptyJSONObject
	}
	return s
}

// rawToJSONString JSON 原始字节 → 字符串（空兜底 "{}"）.
// [EN] Convert raw JSON bytes to a string (empty falls back to "{}").
func rawToJSONString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return emptyJSONObject
	}
	return string(raw)
}
