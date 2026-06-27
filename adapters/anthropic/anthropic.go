/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 20:37:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-27 10:18:33
 * @FilePath: \go-llmx\adapters\anthropic\anthropic.go
 * @Description: Anthropic Claude 对话适配器 —— 编排 transport 传输与 wire 编解码.
 * 协议差异（x-api-key 头 / anthropic-version / max_tokens 必填）收口在本文件；
 * 客户端基座与错误映射骨架见核心库 adapter 包；协议细节见 wire.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcanthropic

import (
	"context"
	"fmt"
	"io"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
	"github.com/kamalyes/go-llmx/transport"
)

// Client Anthropic Claude 对话客户端.
// [EN] Anthropic Claude chat client.
type Client struct {
	// adapter.Base 客户端基座（端点/模型/密钥/传输/日志 + 访问器）.
	// [EN] Client base (endpoint/model/key/transport/logger + accessors).
	adapter.Base
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

	// WithBaseURL 设置端点（兼容网关/代理）.
	// [EN] Set the endpoint.
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

// New 构造客户端（opts 可覆盖端点/模型/超时；默认 claude-sonnet-4-5 + 120s）.
// [EN] Build a client (endpoint/model/timeout overridable).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{adapter.NewBase(MessagesPath, apiKey, DefaultModel)}
	c.BaseURL = DefaultBaseURL
	adapter.Apply(c, opts...)
	return c
}

// GenerateContent 实现 llmx.Model（非流式）.
// [EN] Implement llmx.Model (non-streaming).
func (c *Client) GenerateContent(ctx context.Context, messages []llmx.Message, opts ...llmx.Option) (*llmx.Response, error) {
	if err := validateMessages(messages); err != nil {
		return nil, err
	}
	o := llmx.Apply(opts...)
	c.LogModel(ctx, c.ResolveModel(o), "chat", len(messages))

	var wr wireResponse
	if err := c.TC.PostJSON(ctx, c.GetEndpoint(), c.buildRequest(o, messages, false), &wr, c.headers()); err != nil {
		return nil, adapter.MapTransportError(err, anthropicClassifier{})
	}
	// 部分网关 200 状态仍注入 error 字段（配额耗尽 / 内容审核拦截）
	if wr.Error != nil {
		return nil, mapErrorBody(wr.Error.Type, wr.Error.Type+": "+wr.Error.Message)
	}
	// 空内容判定（无块且无思考轨迹）
	choice := decodeChoice(&wr)
	if len(choice.Content) == 0 && choice.Reasoning == "" {
		return nil, llmx.ErrEmptyResponse
	}

	resp := &llmx.Response{
		Model: c.ResolveModel(o),
		Usage: llmx.Usage{
			PromptTokens:     wr.Usage.InputTokens,
			CompletionTokens: wr.Usage.OutputTokens,
			TotalTokens:      wr.Usage.InputTokens + wr.Usage.OutputTokens,
		},
		Choices: []llmx.Choice{choice},
	}
	return resp, nil
}

// StreamGenerateContent 实现 llmx.Model（SSE 流式）.
// [EN] Implement llmx.Model (SSE streaming).
//
// stream 为 nil 时退化为非流式；handler 返回 ErrStopStream 提前终止
func (c *Client) StreamGenerateContent(ctx context.Context, messages []llmx.Message, stream llmx.StreamHandler, opts ...llmx.Option) (*llmx.Response, error) {
	if err := validateMessages(messages); err != nil {
		return nil, err
	}
	if stream == nil {
		return c.GenerateContent(ctx, messages, opts...)
	}

	o := llmx.Apply(opts...)
	c.LogModel(ctx, c.ResolveModel(o), "stream", len(messages))
	st := newStreamAggregator()

	err := c.TC.DoStream(ctx, transport.MethodPost, c.GetEndpoint(),
		c.buildRequest(o, messages, true), c.headers(),
		func(ev transport.SSEEvent) error {
			return st.feed(ev, stream)
		})
	if err != nil {
		return st.response(c.ResolveModel(o)), adapter.MapTransportError(err, anthropicClassifier{})
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
	msgs, system := encodeMessages(messages)
	req := &wireRequest{
		Model:     c.ResolveModel(o),
		MaxTokens: DefaultMaxTokens, // 协议必填，无配置走默认
		System:    system,
		Stream:    stream,
		Stop:      o.Stop,
		Messages:  msgs,
	}
	if o.MaxTokens > 0 {
		req.MaxTokens = o.MaxTokens
	}
	if o.Temperature != 0 {
		req.Temperature = o.Temperature
	}
	if o.TopP > 0 {
		req.TopP = o.TopP
	}
	for _, t := range o.Tools {
		req.Tools = append(req.Tools, wireTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}
	req.Thinking = encodeThinking(o, req.MaxTokens)
	return req
}

// encodeThinking 思考配置 → wire 参数（budget 显式值优先，否则按档位推导；协议要求预算 <= maxTokens）.
// [EN] Encode the thinking config (explicit budget first, then level-derived).
func encodeThinking(o *llmx.Options, maxTokens int) *wireThinking {
	if o.Thinking == nil || o.Thinking.Mode == llmx.ThinkingNone {
		return nil
	}
	budget := o.Thinking.BudgetTokens
	if budget <= 0 {
		budget = llmx.CalculateThinkingBudget(o.Thinking.Mode, o.MaxTokens)
	}
	if budget < llmx.MinThinkingBudget {
		budget = llmx.MinThinkingBudget
	}
	// 协议约束：思考预算不得超过 max_tokens，超出时钳制
	if maxTokens > 0 && budget > maxTokens {
		budget = maxTokens
	}
	return &wireThinking{Type: thinkingEnabled, BudgetTokens: budget}
}

// validateMessages 空消息快速失败（发起网络请求前拦截）.
// [EN] Fast-fail on empty messages (before any network call).
func validateMessages(messages []llmx.Message) error {
	if len(messages) == 0 {
		return fmt.Errorf("%w: messages must not be empty", llmx.ErrInvalidRequest)
	}
	return nil
}

// headers 认证与协议头（Anthropic 专用：x-api-key + anthropic-version）.
// [EN] Auth and protocol headers (Anthropic-specific).
func (c *Client) headers() map[string]string {
	h := map[string]string{headerVersion: APIVersion}
	if c.APIKey != "" {
		h[headerAPIKey] = c.APIKey
	}
	return h
}

// errStopReading SSE 流正常终止信号（ReadSSE 以 io.EOF 语义收尾）.
// [EN] Normal SSE termination signal (ReadSSE treats io.EOF as done).
var errStopReading = io.EOF

// 编译期断言：实现 llmx.Model 契约.
// [EN] Compile-time assertion of the llmx.Model contract.
var _ llmx.Model = (*Client)(nil)
