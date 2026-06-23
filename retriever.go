/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-23 22:26:39
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-23 22:37:58
 * @FilePath: \go-llmx\retriever.go
 * @Description: 检索器抽象 —— Retriever 契约与 VectorStore 适配.
 * RAG 链路的检索侧收口：QA 链只依赖本接口，不感知向量库与嵌入实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import "context"

// Retriever 检索器（查询 → 相关文档）.
// [EN] Retriever (query to relevant documents).
type Retriever interface {
	// GetRelevantDocuments 检索与查询相关的文档.
	// [EN] Retrieve documents relevant to the query.
	GetRelevantDocuments(ctx context.Context, query string) ([]Document, error)
}

// 默认检索条数（Retriever.K 未设置时）.
// [EN] Default retrieval count.
const DefaultRetrievalK = 3

// VectorStoreRetriever 向量库检索器（Retriever 的 VectorStore 适配）.
// [EN] Vector-store retriever (the VectorStore adapter of Retriever).
type VectorStoreRetriever struct {
	// VS 目标向量库.
	// [EN] Target vector store.
	VS VectorStore

	// Emb 查询嵌入器.
	// [EN] Query embedder.
	Emb Embedder

	// K 检索条数（<=0 走 DefaultRetrievalK）.
	// [EN] Retrieval count.
	K int

	// Filters 元数据过滤（可选）.
	// [EN] Optional metadata filters.
	Filters []Filter
}

// NewRetriever 构造向量库检索器.
// [EN] Build a vector-store retriever.
func NewRetriever(vs VectorStore, emb Embedder, k int) *VectorStoreRetriever {
	return &VectorStoreRetriever{VS: vs, Emb: emb, K: k}
}

// GetRelevantDocuments 实现 Retriever（查询嵌入 → Top-K 相似检索）.
// [EN] Implement Retriever (embed the query, then top-K search).
func (r *VectorStoreRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]Document, error) {
	if r.VS == nil || r.Emb == nil {
		return nil, ErrInvalidRequest
	}
	k := r.K
	if k <= 0 {
		k = DefaultRetrievalK
	}
	vec, err := r.Emb.EmbedQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	return r.VS.SimilaritySearch(ctx, vec, k, r.Filters...)
}

// compile-time assertion: Retriever 实现.
// [EN] Compile-time assertion.
var _ Retriever = (*VectorStoreRetriever)(nil)
