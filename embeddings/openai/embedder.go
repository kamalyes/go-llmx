/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 22:37:00
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:38:16
 * @FilePath: \go-llmx\embeddings\openai\embedder.go
 * @Description: OpenAI 兼容嵌入适配器 —— /embeddings 协议，覆盖 OpenAI 及
 * 兼容网关. 独立子包实现 llmx.Embedder（与对话客户端独立配置端点/模型）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcembed

import (
	"context"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// Client OpenAI 兼容嵌入客户端.
// [EN] OpenAI-compatible embedding client.
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
	// WithAPIKey 设置 API Key.
	// [EN] Set the API key.
	WithAPIKey = adapter.WithAPIKey

	// WithBaseURL 设置兼容端点.
	// [EN] Set a compatible endpoint.
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

// New 构造客户端（opts 可覆盖端点/模型/超时；默认 text-embedding-3-small）.
// [EN] Build a client (endpoint/model/timeout overridable).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{Base: adapter.NewBase(EmbeddingsPath, apiKey, DefaultModel)}
	c.BaseURL = DefaultBaseURL
	adapter.Apply(c, opts...)
	return c
}

// EmbedDocuments 实现 llmx.Embedder（批量嵌入，索引阶段）.
// [EN] Implement llmx.Embedder (batch embedding, indexing phase).
func (c *Client) EmbedDocuments(ctx context.Context, texts []string) ([][]float64, error) {
	c.LogModel(ctx, c.Model, "embed", len(texts))

	var wr wireResponse
	if err := c.TC.PostJSON(ctx, c.GetEndpoint(), &wireRequest{
		Model: c.Model,
		Input: texts,
	}, &wr, c.headers()); err != nil {
		return nil, adapter.MapTransportError(err, classifier{})
	}

	// 部分网关 200 状态仍注入 error 字段
	if wr.Error != nil {
		return nil, adapter.WrapErrorBody(toErrorBody(wr.Error))
	}

	// 响应按 index 归位（协议保证 index 与输入顺序对应）
	vectors := make([][]float64, len(texts))
	for _, d := range wr.Data {
		if d.Index >= 0 && d.Index < len(vectors) {
			vectors[d.Index] = d.Embedding
		}
	}
	return vectors, nil
}

// EmbedQuery 实现 llmx.Embedder（单条嵌入，检索阶段）.
// [EN] Implement llmx.Embedder (single embedding, retrieval phase).
func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float64, error) {
	vectors, err := c.EmbedDocuments(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

// headers 认证头（Bearer 形态）.
// [EN] Auth headers (Bearer style).
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
