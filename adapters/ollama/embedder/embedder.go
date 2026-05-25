/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 22:37:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-25 21:53:00
 * @FilePath: \go-llmx\adapters\ollama\embedder\embedder.go
 * @Description: Ollama 原生嵌入适配器 —— /api/embed 协议.
 * 独立子包实现 llmx.Embedder（与对话客户端独立配置端点/模型）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollamaembed

import (
	"context"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// Client Ollama 原生嵌入客户端.
// [EN] Ollama native embedding client.
type Client struct {
	// adapter.Base 客户端基座（端点/模型/密钥/传输/日志 + 访问器）.
	// [EN] Client base (endpoint/model/key/transport/logger + accessors).
	adapter.Base
}

// Adapter 实现 adapter.HasBase（泛型选项定位基座）.
// [EN] Implement adapter.HasBase (locates the base for generic options).
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

	// WithModel 设置嵌入模型.
	// [EN] Set the embedding model.
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

// New 构造客户端（opts 可覆盖端点/模型/超时；默认本地 11434 + nomic-embed-text）.
// [EN] Build a client (endpoint/model/timeout overridable).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{Base: adapter.NewBase(EmbedPath, apiKey, DefaultModel)}
	c.BaseURL = DefaultBaseURL
	adapter.Apply(c, opts...)
	return c
}

// EmbedDocuments 实现 llmx.Embedder（批量嵌入，索引阶段；input 以数组形态发送）.
// [EN] Implement llmx.Embedder (batch embedding; input as array).
func (c *Client) EmbedDocuments(ctx context.Context, texts []string) ([][]float64, error) {
	return c.embed(ctx, texts)
}

// EmbedQuery 实现 llmx.Embedder（单条嵌入，检索阶段；input 以单字符串形态发送）.
// [EN] Implement llmx.Embedder (single embedding; input as string).
func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float64, error) {
	vectors, err := c.embed(ctx, text)
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, llmx.ErrEmptyResponse
	}
	return vectors[0], nil
}

// embed 统一嵌入入口（协议 input 两形态收口：string 单条 / []string 批量）.
// [EN] Unified embedding entry (input shape: string / []string).
func (c *Client) embed(ctx context.Context, input any) ([][]float64, error) {
	n := 1
	if texts, ok := input.([]string); ok {
		n = len(texts)
	}
	c.LogModel(ctx, c.Model, "embed", n)

	var wr wireResponse
	if err := c.TC.PostJSON(ctx, c.GetEndpoint(), &wireRequest{
		Model: c.Model,
		Input: input,
	}, &wr, c.headers()); err != nil {
		return nil, adapter.MapTransportError(err, classifier{})
	}

	// Ollama 任意状态均可能以 error 字段返回（模型未拉取等）
	if wr.Error != "" {
		return nil, adapter.WrapErrorBody(&adapter.ErrorBody{Type: errorTypeOllamaEmbed, Message: wr.Error})
	}
	return wr.Embeddings, nil
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

// 编译期断言：实现 llmx.Embedder 契约.
// [EN] Compile-time assertion of the llmx.Embedder contract.
var _ llmx.Embedder = (*Client)(nil)
