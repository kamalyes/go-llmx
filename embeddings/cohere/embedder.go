/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-11 21:02:19
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-18 10:58:53
 * @FilePath: \go-llmx\embeddings\cohere\embedder.go
 * @Description: Cohere 嵌入适配器 —— /v2/embed 协议（embed-v4.0）.
 * 检索语义非对称：索引走 search_document、查询走 search_query（模型分开优化）；
 * 向量仅消费 float 形态（量化形态不适用通用检索）；
 * 单请求 96 条上限自动拆批有界并行
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccembed

import (
	"context"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// Client Cohere 嵌入客户端.
// [EN] Cohere embedding client.
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

	// WithHTTPClient 注入自定义 http.Client.
	// [EN] Inject a custom http.Client.
	WithHTTPClient = adapter.WithHTTPClient

	// WithLogger 注入日志（缺省静默）.
	// [EN] Inject a logger.
	WithLogger = adapter.WithLogger
)

// New 构造客户端（默认 embed-v4.0）.
// [EN] Build a client (embed-v4.0 by default).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{Base: adapter.NewBase(EmbedPath, apiKey, DefaultModel)}
	c.BaseURL = DefaultBaseURL
	adapter.Apply(c, opts...)
	return c
}

// headers 认证头（Bearer 形态，委托基座共享缓存）.
// [EN] Auth headers (Bearer, delegated to the base cache).
func (c *Client) headers() map[string]string {
	return c.BearerHeaders()
}

// embedRange 单批走 /v2/embed 端点（input_type 区分索引/查询语义；
// 结果按输入顺序写入 out[start:]）.
// [EN] One /v2/embed batch (input_type distinguishes indexing from querying;
// results written in order into out[start:]).
func (c *Client) embedRange(ctx context.Context, texts []string, inputType string, out [][]float64, start int) error {
	var wr wireResponse
	if err := c.TC.PostJSON(ctx, c.GetEndpoint(), &wireRequest{
		Model:          c.Model,
		Texts:          texts,
		InputType:      inputType,
		EmbeddingTypes: []string{embeddingTypesFloats},
	}, &wr, c.headers()); err != nil {
		return adapter.MapTransportError(err, classifier{})
	}
	if len(wr.Embeddings.Float) != len(texts) {
		return adapter.ErrVectorCountMismatch(len(wr.Embeddings.Float), len(texts))
	}
	for i, v := range wr.Embeddings.Float {
		out[start+i] = v
	}
	return nil
}

// EmbedDocuments 实现 llmx.Embedder（批量嵌入，索引阶段，input_type=search_document；
// 超出单请求上限 96 条自动拆批有界并行，各批写入互斥区间结果有序）.
// [EN] Implement llmx.Embedder (batch, indexing phase; beyond the 96-text
// per-request limit batches dispatch bounded-parallel, disjoint writes
// keep the result ordered).
func (c *Client) EmbedDocuments(ctx context.Context, texts []string) ([][]float64, error) {
	if err := adapter.ValidateEmbedTexts(texts); err != nil {
		return nil, err
	}
	c.LogModel(ctx, c.Model, "embed", len(texts))

	vectors := make([][]float64, len(texts))
	if err := adapter.ParallelBatches(ctx, len(texts), embedBatchMaxTexts, adapter.EmbedBatchWorkers,
		func(ctx context.Context, start, end int) error {
			return c.embedRange(ctx, texts[start:end], inputTypeDocument, vectors, start)
		}); err != nil {
		return nil, err
	}
	return vectors, nil
}

// EmbedQuery 实现 llmx.Embedder（单条嵌入，检索阶段，input_type=search_query）.
// [EN] Implement llmx.Embedder (single, retrieval phase).
func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float64, error) {
	c.LogModel(ctx, c.Model, "embed", 1)
	out := make([][]float64, 1)
	if err := c.embedRange(ctx, []string{text}, inputTypeQuery, out, 0); err != nil {
		return nil, err
	}
	return out[0], nil
}

// 编译期断言：实现 llmx.Embedder 契约.
// [EN] Compile-time assertion of the llmx.Embedder contract.
var _ llmx.Embedder = (*Client)(nil)
