/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 20:19:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 20:25:31
 * @FilePath: \go-llmx\adapters\mistral\mistral.go
 * @Description: Mistral 适配器 —— Chat Completions 协议实现（含 SSE 流式）.
 * 编排 transport 传输与 wire 编解码，与 openai 适配器同构但协议差异独立收口
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmistral

import (
	"context"
	"fmt"
	"io"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
	"github.com/kamalyes/go-llmx/transport"
)

// Client Mistral 客户端.
// [EN] Mistral client.
type Client struct {
	// adapter.Base 客户端基座.
	// [EN] Client base.
	adapter.Base
}

// Adapter 实现 adapter.hasBase.
// [EN] Implement adapter.hasBase.
func (c *Client) Adapter() *adapter.Base { return &c.Base }

// Option 客户端构造选项（复用 adapter 多态体系）.
// [EN] Client constructor option.
type Option = adapter.Option

// 构造选项（adapter 公共选项直通导出）.
// [EN] Constructor options (re-exported from adapter).
var (
	// WithAPIKey 设置 API Key.
	// [EN] Set the API key.
	WithAPIKey = adapter.WithAPIKey

	// WithBaseURL 设置端点.
	// [EN] Set the endpoint.
	WithBaseURL = adapter.WithBaseURL

	// WithModel 设置默认模型.
	// [EN] Set the default model.
	WithModel = adapter.WithModel

	// WithTimeout 设置请求总超时.
	// [EN] Set the total request timeout.
	WithTimeout = adapter.WithTimeout

	// WithHTTPClient 注入自定义 http.Client.
	// [EN] Inject a custom http.Client.
	WithHTTPClient = adapter.WithHTTPClient

	// WithLogger 注入日志（缺省静默）.
	// [EN] Inject a logger.
	WithLogger = adapter.WithLogger
)

// New 构造客户端（默认 mistral-small-latest）.
// [EN] Build a client.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{Base: adapter.NewBase(ChatCompletionsPath, apiKey, DefaultModel)}
	c.BaseURL = DefaultBaseURL
	adapter.Apply(c, opts...)
	return c
}

// headers 认证头（Bearer 形态）.
// [EN] Auth headers.
func (c *Client) headers() map[string]string {
	return map[string]string{"Authorization": "Bearer " + c.APIKey}
}

// GenerateContent 实现 llmx.Model（非流式）.
// [EN] Implement llmx.Model (non-streaming).
func (c *Client) GenerateContent(ctx context.Context, messages []llmx.Message, opts ...llmx.Option) (*llmx.Response, error) {
	if err := validateMessages(messages); err != nil {
		return nil, err
	}
	o := llmx.Apply(opts...)
	model := c.ResolveModel(o)
	c.LogModel(ctx, model, "chat", len(messages))

	var wr wireResponse
	req := buildRequest(o, messages, false)
	req.Model = model
	if err := c.TC.PostJSON(ctx, c.GetEndpoint(), req, &wr, c.headers()); err != nil {
		return nil, adapter.MapTransportError(err, mistralClassifier{})
	}
	if len(wr.Choices) == 0 {
		return nil, llmx.ErrEmptyResponse
	}
	return decodeResponse(&wr, model), nil
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
	model := c.ResolveModel(o)
	c.LogModel(ctx, model, "stream", len(messages))
	st := newStreamAggregator()

	req := buildRequest(o, messages, true)
	req.Model = model
	err := c.TC.DoStream(ctx, transport.MethodPost, c.GetEndpoint(), req, c.headers(),
		func(ev transport.SSEEvent) error {
			if transport.IsDoneMarker(ev.Data) {
				return errStopReading
			}
			return st.feed(ev.Data, stream)
		})
	if err != nil {
		return st.response(model), adapter.MapTransportError(err, mistralClassifier{})
	}
	resp := st.response(model)
	if len(resp.Choices) == 0 {
		return resp, llmx.ErrEmptyResponse
	}
	return resp, nil
}

// validateMessages 入参校验.
// [EN] Validate inputs.
func validateMessages(messages []llmx.Message) error {
	if len(messages) == 0 {
		return fmt.Errorf("%w: empty messages", llmx.ErrInvalidRequest)
	}
	for _, m := range messages {
		if len(m.Content) == 0 && m.Role != llmx.RoleTool {
			return fmt.Errorf("%w: empty content in %s message", llmx.ErrInvalidRequest, m.Role)
		}
	}
	return nil
}

// errStopReading SSE 流正常终止信号（ReadSSE 以 io.EOF 语义收尾）.
// [EN] Normal SSE termination (ReadSSE treats it as io.EOF).
var errStopReading = io.EOF
