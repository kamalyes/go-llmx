/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-01 20:28:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-21 20:58:02
 * @FilePath: \go-llmx\adapters\openai\openai.go
 * @Description: OpenAI 兼容对话适配器 —— 编排 transport 传输与 wire 编解码.
 * 覆盖 OpenAI / DeepSeek / OpenRouter / Groq / vLLM 等兼容端点（WithBaseURL 切换）；
 * 客户端基座与错误映射骨架见核心库 adapter 包；协议细节见 wire.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"context"
	"io"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
	"github.com/kamalyes/go-llmx/transport"
)

// Client OpenAI 兼容客户端（对话 + 嵌入双能力）.
// [EN] OpenAI-compatible client (chat + embedding).
type Client struct {
	// adapter.Base 客户端基座（端点/模型/密钥/传输/日志 + 访问器）.
	// [EN] Client base (endpoint/model/key/transport/logger + accessors).
	adapter.Base

	// embedModel 嵌入模型（与对话模型独立管理）.
	// [EN] Embedding model (managed independently of the chat model).
	embedModel string
}

// Adapter 实现 adapter.hasBase（泛型选项定位基座）.
// [EN] Implement adapter.hasBase (locates the base for generic options).
func (c *Client) Adapter() *adapter.Base { return &c.Base }

// Option 客户端构造选项（复用 adapter 多态体系）.
// [EN] Client constructor option (shared adapter polymorphism).
type Option = adapter.Option

// 构造选项（adapter 公共选项直通导出，用法与本地定义一致）.
// [EN] Constructor options (re-exported from adapter).
var (
	// WithAPIKey 设置 API Key.
	// [EN] Set the API key.
	WithAPIKey = adapter.WithAPIKey

	// WithBaseURL 设置兼容端点（DeepSeek/OpenRouter/Groq/vLLM 等）.
	// [EN] Set a compatible endpoint.
	WithBaseURL = adapter.WithBaseURL

	// WithModel 设置默认模型.
	// [EN] Set the default model.
	WithModel = adapter.WithModel

	// WithTimeout 设置请求总超时.
	// [EN] Set the total request timeout.
	WithTimeout = adapter.WithTimeout

	// WithHTTPClient 注入自定义 http.Client（代理/连接池调优）.
	// [EN] Inject a custom http.Client.
	WithHTTPClient = adapter.WithHTTPClient

	// WithLogger 注入日志（缺省静默）.
	// [EN] Inject a logger.
	WithLogger = adapter.WithLogger
)

// New 构造客户端（opts 可覆盖端点/模型/超时；默认 gpt-4o-mini + 120s）.
// [EN] Build a client (endpoint/model/timeout overridable).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{Base: adapter.NewBase(ChatCompletionsPath, apiKey, DefaultModel)}
	c.BaseURL = DefaultBaseURL
	adapter.Apply(c, opts...)
	return c
}

// GenerateContent 实现 llmx.Model（非流式）.
// [EN] Implement llmx.Model (non-streaming).
func (c *Client) GenerateContent(ctx context.Context, messages []llmx.Message, opts ...llmx.Option) (*llmx.Response, error) {
	o := llmx.Apply(opts...)
	c.LogModel(ctx, c.ResolveModel(o), "chat", len(messages))

	var wr wireResponse
	if err := c.TC.PostJSON(ctx, c.GetEndpoint(), c.buildRequest(o, messages, false), &wr, c.headers()); err != nil {
		return nil, adapter.MapTransportError(err, openaiClassifier{})
	}
	// 部分网关 200 状态仍注入 error 字段（配额耗尽 / 内容审核拦截）
	if wr.Error != nil {
		return nil, adapter.WrapErrorBody(toErrorBody(wr.Error))
	}

	resp := &llmx.Response{
		Model: c.ResolveModel(o),
		Usage: llmx.Usage{
			PromptTokens:     wr.Usage.PromptTokens,
			CompletionTokens: wr.Usage.CompletionTokens,
			TotalTokens:      wr.Usage.TotalTokens,
		},
	}
	for _, ch := range wr.Choices {
		resp.Choices = append(resp.Choices, decodeChoice(ch))
	}
	if len(resp.Choices) == 0 {
		return nil, llmx.ErrEmptyResponse
	}
	return resp, nil
}

// StreamGenerateContent 实现 llmx.Model（SSE 流式）.
// [EN] Implement llmx.Model (SSE streaming).
//
// stream 为 nil 时退化为非流式；handler 返回 ErrStopStream 提前终止
func (c *Client) StreamGenerateContent(ctx context.Context, messages []llmx.Message, stream llmx.StreamHandler, opts ...llmx.Option) (*llmx.Response, error) {
	if stream == nil {
		return c.GenerateContent(ctx, messages, opts...)
	}

	o := llmx.Apply(opts...)
	c.LogModel(ctx, c.ResolveModel(o), "stream", len(messages))
	st := newStreamAggregator()

	err := c.TC.DoStream(ctx, transport.MethodPost, c.GetEndpoint(),
		c.buildRequest(o, messages, true), c.headers(),
		func(ev transport.SSEEvent) error {
			if transport.IsDoneMarker(ev.Data) {
				return errStopReading
			}
			if err := st.feed(ev.Data, stream); err != nil {
				return err
			}
			return nil
		})
	if err != nil {
		return st.response(c.ResolveModel(o)), adapter.MapTransportError(err, openaiClassifier{})
	}

	resp := st.response(c.ResolveModel(o))
	if len(resp.Choices) == 0 {
		return resp, llmx.ErrEmptyResponse
	}
	return resp, nil
}

// ============================================================================
// 内部：请求组装
// ============================================================================

// buildRequest 组装 wire 请求（消费 Options 中适配器支持的字段）.
// [EN] Assemble the wire request from options.
func (c *Client) buildRequest(o *llmx.Options, messages []llmx.Message, stream bool) *wireRequest {
	req := &wireRequest{
		Model:    c.ResolveModel(o),
		Stream:   stream,
		Stop:     o.Stop,
		User:     o.User,
		Messages: encodeMessages(messages),
	}
	if o.Temperature != 0 {
		req.Temperature = o.Temperature
	}
	if o.MaxTokens > 0 {
		req.MaxTokens = o.MaxTokens
	}
	if o.TopP > 0 {
		req.TopP = o.TopP
	}
	if o.N > 1 {
		req.N = o.N
	}
	if o.Seed != 0 {
		s := o.Seed
		req.Seed = &s
	}
	if o.JSONMode {
		req.ResponseFormat = &wireResponseFormat{Type: responseFormatJSONObject}
	}
	for _, t := range o.Tools {
		req.Tools = append(req.Tools, wireTool{
			Type: toolTypeFunction,
			Function: wireToolSchema{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
				Strict:      t.Strict,
			},
		})
	}
	req.ReasoningEffort = encodeReasoningEffort(o)
	return req
}

// encodeReasoningEffort 思考档位 → OpenAI reasoning_effort 参数（o 系模型消费；
// 仅 low/medium/high 有协议意义，none/auto 不携带由模型默认行为决定）.
// [EN] Map the thinking level to the OpenAI reasoning_effort parameter.
func encodeReasoningEffort(o *llmx.Options) string {
	if o.Thinking == nil {
		return ""
	}
	switch o.Thinking.Mode {
	case llmx.ThinkingLow, llmx.ThinkingMedium, llmx.ThinkingHigh:
		return string(o.Thinking.Mode)
	default:
		return ""
	}
}

// headers 认证与协议头（Bearer 形态）.
// [EN] Auth and protocol headers (Bearer style).
func (c *Client) headers() map[string]string {
	h := map[string]string{}
	if c.APIKey != "" {
		h["Authorization"] = "Bearer " + c.APIKey
	}
	return h
}

// errStopReading SSE 流正常终止信号（ReadSSE 以 io.EOF 语义收尾）.
// [EN] Normal SSE termination signal (ReadSSE treats io.EOF as done).
var errStopReading = io.EOF

// 编译期断言：实现 llmx.Model 契约.
// [EN] Compile-time assertion of the llmx.Model contract.
var _ llmx.Model = (*Client)(nil)
