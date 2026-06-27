/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 20:35:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-27 10:18:33
 * @FilePath: \go-llmx\adapters\ollama\ollama.go
 * @Description: Ollama 本地推理对话适配器 —— 编排 transport 传输与 wire 编解码.
 * NDJSON 流式（transport.DoNDJSON）；客户端基座与错误映射骨架见核心库 adapter 包；
 * 协议细节见 wire.go，流式聚合见 stream.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollama

import (
	"context"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
	"github.com/kamalyes/go-llmx/transport"
)

// Client Ollama 对话客户端（嵌入能力见 embedder 子包）.
// [EN] Ollama chat client (embedding in the embedder subpackage).
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
	// WithAPIKey 设置 API Key（本地部署可空，代理网关必需）.
	// [EN] Set the API key (empty for local, required for gateways).
	WithAPIKey = adapter.WithAPIKey

	// WithBaseURL 设置端点（远程 Ollama / 反向代理）.
	// [EN] Set the endpoint (remote Ollama / reverse proxy).
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

// New 构造客户端（opts 可覆盖端点/模型/超时；默认本地 11434 + llama3.2）.
// [EN] Build a client (endpoint/model/timeout overridable).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{Base: adapter.NewBase(ChatPath, apiKey, DefaultModel)}
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
		return nil, adapter.MapTransportError(err, ollamaClassifier{})
	}
	// 任意状态均可能以 error 字段返回（本地进程错误/模型未拉取等）
	if wr.Error != "" {
		return nil, adapter.WrapErrorBody(&adapter.ErrorBody{Type: errorTypeOllama, Message: wr.Error})
	}

	resp := &llmx.Response{
		Model: c.ResolveModel(o),
		Usage: usage(&wr),
	}
	choice := decodeChoice(&wr)
	resp.Choices = append(resp.Choices, choice)
	if len(choice.Content) == 0 {
		return resp, llmx.ErrEmptyResponse
	}
	return resp, nil
}

// StreamGenerateContent 实现 llmx.Model（NDJSON 流式）.
// [EN] Implement llmx.Model (NDJSON streaming).
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

	err := c.TC.DoNDJSON(ctx, transport.MethodPost, c.GetEndpoint(),
		c.buildRequest(o, messages, true), c.headers(),
		func(line string) error {
			return st.feed(line, stream)
		})
	if err != nil {
		return st.response(c.ResolveModel(o)), adapter.MapTransportError(err, ollamaClassifier{})
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

// buildRequest 组装 wire 请求（Options 中适配器支持的字段映射到 options 容器）.
// [EN] Assemble the wire request (options mapped into the protocol container).
func (c *Client) buildRequest(o *llmx.Options, messages []llmx.Message, stream bool) *wireRequest {
	req := &wireRequest{
		Model:    c.ResolveModel(o),
		Stream:   stream,
		Messages: encodeMessages(messages),
	}

	// 采样参数映射（协议收拢在 options 容器；全部零值时不挂载）
	opts := &wireOptions{
		Temperature: o.Temperature,
		TopP:        o.TopP,
		NumPredict:  o.MaxTokens,
		Seed:        o.Seed,
		Stop:        o.Stop,
	}
	if opts.Temperature != 0 || opts.TopP != 0 || opts.NumPredict > 0 || opts.Seed != 0 || len(opts.Stop) > 0 {
		req.Options = opts
	}

	if o.JSONMode {
		req.Format = formatJSON
	}
	for _, t := range o.Tools {
		req.Tools = append(req.Tools, wireTool{
			Type: toolTypeFunction,
			Function: wireToolSchema{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return req
}

// validateMessages 空消息快速失败（发起网络请求前拦截）.
// [EN] Fast-fail on empty messages (before any network call).
func validateMessages(messages []llmx.Message) error {
	if len(messages) == 0 {
		return fmt.Errorf("%w: messages must not be empty", llmx.ErrInvalidRequest)
	}
	return nil
}

// headers 认证头（本地部署可空，代理网关 Bearer 形态）.
// [EN] Auth headers (empty locally, Bearer for gateways).
func (c *Client) headers() map[string]string {
	h := map[string]string{}
	if c.APIKey != "" {
		h["Authorization"] = "Bearer " + c.APIKey
	}
	return h
}

// 编译期断言：实现 llmx.Model 契约.
// [EN] Compile-time assertion of the llmx.Model contract.
var _ llmx.Model = (*Client)(nil)
