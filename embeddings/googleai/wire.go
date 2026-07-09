/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-09 21:05:12
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:05:12
 * @FilePath: \go-llmx\embeddings\googleai\wire.go
 * @Description: Google AI 嵌入协议编解码 —— embedContent / batchEmbedContents wire 结构
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgembed

// wireBatchRequest batchEmbedContents 请求体.
// [EN] batchEmbedContents request body.
type wireBatchRequest struct {
	// Requests 嵌入请求集合（单批上限 100，超出由客户端自动分批）.
	// [EN] Embedding requests (max 100 per batch; client auto-splits).
	Requests []wireEmbedRequest `json:"requests"`
}

// wireEmbedRequest 单条嵌入请求（content + 任务类型）.
// [EN] A single embedding request (content + task type).
type wireEmbedRequest struct {
	// Content 文本内容（parts 形态与对话协议同构）.
	// [EN] Text content (parts shape shared with the chat protocol).
	Content wireContent `json:"content"`

	// TaskType 任务类型（索引 RETRIEVAL_DOCUMENT / 查询 RETRIEVAL_QUERY）.
	// [EN] Task type.
	TaskType string `json:"taskType,omitempty"`
}

// wireContent 文本内容载体.
// [EN] Text content carrier.
type wireContent struct {
	// Parts 文本分段（单条嵌入恒为一段）.
	// [EN] Text segments (always one for a single embedding).
	Parts []wirePart `json:"parts"`
}

// wirePart 单个文本分段.
// [EN] A single text segment.
type wirePart struct {
	// Text 文本.
	// [EN] The text.
	Text string `json:"text"`
}

// wireSingleResponse embedContent 响应体（单条）.
// [EN] embedContent response body (single).
type wireSingleResponse struct {
	// Embedding 嵌入向量.
	// [EN] The embedding vector.
	Embedding wireValues `json:"embedding"`
}

// wireBatchResponse batchEmbedContents 响应体（批量）.
// [EN] batchEmbedContents response body (batch).
type wireBatchResponse struct {
	// Embeddings 嵌入向量集合（顺序与请求一致，无 index 字段）.
	// [EN] Embedding vectors (order matches the requests; no index field).
	Embeddings []wireValues `json:"embeddings"`
}

// wireValues 向量载荷.
// [EN] Vector payload.
type wireValues struct {
	// Values 向量分量.
	// [EN] Vector components.
	Values []float64 `json:"values"`
}
