/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 21:17:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 21:17:00
 * @FilePath: \go-llmx\embedder.go
 * @Description: 向量嵌入抽象 —— RAG 检索链路的文本向量化接口，
 * 具体实现见各厂商适配器的 embedder 子包（openai / ollama）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package llmx

import "context"

// Embedder 向量嵌入器（文档索引与查询检索共用契约）.
// [EN] Text embedder (shared contract for indexing and retrieval).
type Embedder interface {
	// EmbedDocuments 批量嵌入文档（索引阶段）.
	// [EN] Embed documents in batch (indexing phase).
	EmbedDocuments(ctx context.Context, texts []string) ([][]float64, error)

	// EmbedQuery 嵌入单条查询（检索阶段）.
	// [EN] Embed a single query (retrieval phase).
	EmbedQuery(ctx context.Context, text string) ([]float64, error)
}
