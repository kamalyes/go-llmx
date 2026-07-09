/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 22:15:00
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:38:16
 * @FilePath: \go-llmx\embeddings\ollama\wire.go
 * @Description: Ollama 嵌入协议编解码 —— /api/embed 请求/响应结构.
 * input 兼容单字符串与数组两形态；embeddings 按输入顺序对应（无 index 字段）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollamaembed

// wireRequest /api/embed 请求体.
// [EN] /api/embed request body.
type wireRequest struct {
	// Model 嵌入模型名.
	// [EN] Embedding model name.
	Model string `json:"model"`

	// Input 输入（单条 string 或批量 []string，协议两形态兼容）.
	// [EN] Input (single string or batch []string, both protocol shapes).
	Input any `json:"input"`
}

// wireResponse /api/embed 响应体.
// [EN] /api/embed response body.
type wireResponse struct {
	// Embeddings 向量集合（与输入顺序一一对应，无 index 字段）.
	// [EN] Vectors (order matches input; no index field).
	Embeddings [][]float64 `json:"embeddings"`

	// Error 错误信息（任意状态均可能注入）.
	// [EN] Error message (injected on any status).
	Error string `json:"error,omitempty"`
}
