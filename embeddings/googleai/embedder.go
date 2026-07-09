/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-09 21:09:38
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:09:38
 * @FilePath: \go-llmx\embeddings\googleai\embedder.go
 * @Description: Google AI 嵌入适配器 —— embedContent / batchEmbedContents 协议.
 * 检索语义非对称：索引走 RETRIEVAL_DOCUMENT、查询走 RETRIEVAL_QUERY（模型分开优化）；
 * 批量自动按 100 条分批（官方上限，内部消化不暴露配置）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgembed

import (
	"context"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapter"
)

// Client Google AI 嵌入客户端.
// [EN] Google AI embedding client.
type Client struct {
	// adapter.Base 客户端基座（Path 不使用：模型名拼入 URL）.
	// [EN] Client base (Path unused: model name is embedded in the URL).
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

	// WithBaseURL 设置端点（Vertex AI 网关/代理）.
	// [EN] Set the endpoint.
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

// New 构造客户端（默认 gemini-embedding-001）.
// [EN] Build a client (gemini-embedding-001 by default).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{Base: adapter.NewBase("", apiKey, DefaultModel)}
	c.BaseURL = DefaultBaseURL
	adapter.Apply(c, opts...)
	return c
}

// endpoint 组装完整 URL（模型名拼路径：{base}/models/{model}:method）.
// [EN] Assemble the full URL.
func (c *Client) endpoint(model, method string) string {
	return c.BaseURL + "/models/" + model + method
}

// headers 认证头（x-goog-api-key）.
// [EN] Auth headers.
func (c *Client) headers() map[string]string {
	return map[string]string{"x-goog-api-key": c.APIKey}
}

// EmbedDocuments 实现 llmx.Embedder（批量嵌入，索引阶段，taskType=RETRIEVAL_DOCUMENT）.
// [EN] Implement llmx.Embedder (batch, indexing phase).
//
// 单批上限 100 条（官方限制），超出自动分批顺序提交，返回顺序与输入一致
func (c *Client) EmbedDocuments(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("%w: no texts to embed", llmx.ErrInvalidRequest)
	}
	c.LogModel(ctx, c.Model, "embed", len(texts))

	vectors := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += maxBatchRequests {
		end := start + maxBatchRequests
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]

		req := wireBatchRequest{Requests: make([]wireEmbedRequest, len(batch))}
		for i, text := range batch {
			req.Requests[i] = wireEmbedRequest{
				Content:  wireContent{Parts: []wirePart{{Text: text}}},
				TaskType: taskTypeDocument,
			}
		}

		var wr wireBatchResponse
		url := c.endpoint(c.Model, methodBatchEmbedContents)
		if err := c.TC.PostJSON(ctx, url, req, &wr, c.headers()); err != nil {
			return nil, adapter.MapTransportError(err, classifier{})
		}
		if len(wr.Embeddings) != len(batch) {
			return nil, fmt.Errorf("%w: batch returned %d vectors for %d texts",
				llmx.ErrEmptyResponse, len(wr.Embeddings), len(batch))
		}
		for _, e := range wr.Embeddings {
			vectors = append(vectors, e.Values)
		}
	}
	return vectors, nil
}

// EmbedQuery 实现 llmx.Embedder（单条嵌入，检索阶段，taskType=RETRIEVAL_QUERY）.
// [EN] Implement llmx.Embedder (single, retrieval phase).
func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float64, error) {
	c.LogModel(ctx, c.Model, "embed", 1)

	var wr wireSingleResponse
	req := wireEmbedRequest{
		Content:  wireContent{Parts: []wirePart{{Text: text}}},
		TaskType: taskTypeQuery,
	}
	url := c.endpoint(c.Model, methodEmbedContent)
	if err := c.TC.PostJSON(ctx, url, req, &wr, c.headers()); err != nil {
		return nil, adapter.MapTransportError(err, classifier{})
	}
	if len(wr.Embedding.Values) == 0 {
		return nil, llmx.ErrEmptyResponse
	}
	return wr.Embedding.Values, nil
}

// 编译期断言：实现 llmx.Embedder 契约.
// [EN] Compile-time assertion of the llmx.Embedder contract.
var _ llmx.Embedder = (*Client)(nil)
