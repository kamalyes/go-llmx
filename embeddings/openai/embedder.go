/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 22:37:00
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-18 09:51:38
 * @FilePath: \go-llmx\embeddings\openai\embedder.go
 * @Description: OpenAI 兼容嵌入适配器 —— /embeddings 协议，覆盖 OpenAI 及
 * 兼容网关. 独立子包实现 llmx.Embedder（与对话客户端独立配置端点/模型）；
 * 超批拆分 + 有界并行派发（adapter.ParallelBatches）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcembed

import (
	"context"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// Client OpenAI 兼容嵌入客户端.
// [EN] OpenAI-compatible embedding client.
type Client struct {
	// adapter.Base 客户端基座（端点/模型/密钥/传输/日志 + 访问器）.
	// [EN] Client base (endpoint/model/key/transport/logger + accessors).
	adapter.Base

	// embedBatch 每请求最大条数（超出拆批，默认 DefaultEmbedBatch）.
	// [EN] Max texts per request (split beyond, default DefaultEmbedBatch).
	embedBatch int

	// stripNewLines 压平换行（OpenAI 官方建议，默认开；拷贝副本不改调用方切片）.
	// [EN] Flatten newlines (OpenAI recommendation, default on; copies).
	stripNewLines bool
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

// WithBatchSize 每请求最大批量（超批拆分有界并行；n <= 0 忽略）.
// [EN] Max texts per request (bounded-parallel beyond; non-positive ignored).
func WithBatchSize(n int) Option {
	return func(h adapter.HasBase) {
		if c, ok := h.(*Client); ok && n > 0 {
			c.embedBatch = n
		}
	}
}

// WithStripNewLines 压平换行开关（默认开，对齐 OpenAI 官方建议与
// langchaingo 语义；关闭后原样发送）.
// [EN] Toggle newline flattening (default on, per OpenAI guidance and
// langchaingo semantics; off sends text as-is).
func WithStripNewLines(strip bool) Option {
	return func(h adapter.HasBase) {
		if c, ok := h.(*Client); ok {
			c.stripNewLines = strip
		}
	}
}

// New 构造客户端（opts 可覆盖端点/模型/超时/批量；默认 text-embedding-3-small）.
// [EN] Build a client (endpoint/model/timeout/batch overridable).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		Base:          adapter.NewBase(EmbeddingsPath, apiKey, DefaultModel),
		embedBatch:    DefaultEmbedBatch,
		stripNewLines: true,
	}
	c.BaseURL = DefaultBaseURL
	adapter.Apply(c, opts...)
	return c
}

// EmbedDocuments 实现 llmx.Embedder（批量嵌入，索引阶段；超批拆分有界并行，
// 各批写入互斥区间结果有序）.
// [EN] Implement llmx.Embedder (indexing phase; bounded-parallel batches,
// disjoint writes keep the result ordered).
func (c *Client) EmbedDocuments(ctx context.Context, texts []string) ([][]float64, error) {
	if err := adapter.ValidateEmbedTexts(texts); err != nil {
		return nil, err
	}
	c.LogModel(ctx, c.Model, "embed", len(texts))

	input := texts
	if c.stripNewLines {
		// 拷贝副本压平（不改调用方切片）
		input = make([]string, len(texts))
		for i, t := range texts {
			input[i] = strings.ReplaceAll(t, "\n", " ")
		}
	}

	vectors := make([][]float64, len(texts))
	if err := adapter.ParallelBatches(ctx, len(texts), c.embedBatch, adapter.EmbedBatchWorkers,
		func(ctx context.Context, start, end int) error {
			return c.embedRange(ctx, input[start:end], vectors, start)
		}); err != nil {
		return nil, err
	}
	return vectors, nil
}

// EmbedQuery 实现 llmx.Embedder（单条嵌入，检索阶段；换行压平同样生效）.
// [EN] Implement llmx.Embedder (single embedding, retrieval phase; newline
// flattening applies too).
func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float64, error) {
	vectors, err := c.EmbedDocuments(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

// embedRange 单批请求并按批内 index 归位写入 out[start:]（网关乱序/越界防护）.
// [EN] One batch request, placing vectors by in-batch index into out[start:].
func (c *Client) embedRange(ctx context.Context, texts []string, out [][]float64, start int) error {
	var wr wireResponse
	if err := c.TC.PostJSON(ctx, c.GetEndpoint(), &wireRequest{
		Model: c.Model,
		Input: texts,
	}, &wr, c.headers()); err != nil {
		return adapter.MapTransportError(err, classifier{})
	}

	// 部分网关 200 状态仍注入 error 字段
	if wr.Error != nil {
		return adapter.WrapErrorBody(toErrorBody(wr.Error))
	}
	if len(wr.Data) != len(texts) {
		return adapter.ErrVectorCountMismatch(len(wr.Data), len(texts))
	}

	// 响应按 index 归位（协议保证 index 与输入顺序对应）
	for _, d := range wr.Data {
		if d.Index >= 0 && d.Index < len(texts) {
			out[start+d.Index] = d.Embedding
		}
	}
	return nil
}

// headers 认证头（Bearer 形态，委托基座共享缓存）.
// [EN] Auth headers (Bearer, delegated to the base cache).
func (c *Client) headers() map[string]string {
	return c.BearerHeaders()
}

// 编译期断言：实现 llmx.Embedder 契约.
// [EN] Compile-time assertion of the llmx.Embedder contract.
var _ llmx.Embedder = (*Client)(nil)
