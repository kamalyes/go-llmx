/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 21:06:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 21:06:00
 * @FilePath: \go-llmx\vectorstore.go
 * @Description: 向量存储抽象 —— Document/Filter + VectorStore 接口，
 * 具体实现见 adapters/vectorstores（memory / redis / pgvector）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package llmx

import "context"

// Document 检索文档（内容 + 元数据）.
// [EN] A retrievable document (content + metadata).
type Document struct {
	// PageContent 文档文本内容.
	// [EN] Document text content.
	PageContent string

	// Metadata 元数据（过滤检索用）.
	// [EN] Metadata (for filtered retrieval).
	Metadata map[string]any
}

// Filter 元数据等值过滤（单 Filter 内多 KV 为 AND，多 Filter 之间亦为 AND）.
// [EN] Metadata equality filter (AND within and across filters).
type Filter map[string]any

// VectorStore 向量存储.
// [EN] Vector store.
type VectorStore interface {
	// AddDocuments 写入文档与向量.
	// [EN] Add documents with their vectors.
	AddDocuments(ctx context.Context, docs []Document, vectors [][]float64) error

	// SimilaritySearch 相似度检索 Top-K（filters 可选）.
	// [EN] Similarity search for top-K (filters optional).
	SimilaritySearch(ctx context.Context, query []float64, topK int, filters ...Filter) ([]Document, error)
}
