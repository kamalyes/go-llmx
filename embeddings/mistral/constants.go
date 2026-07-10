/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-10 21:12:26
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-10 21:12:26
 * @FilePath: \go-llmx\embeddings\mistral\constants.go
 * @Description: Mistral 嵌入适配器常量 —— 默认端点/模型/协议路径
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmembed

const (
	// DefaultBaseURL Mistral 官方端点.
	// [EN] Mistral official endpoint.
	DefaultBaseURL = "https://api.mistral.ai/v1"

	// DefaultModel 默认嵌入模型.
	// [EN] Default embedding model.
	DefaultModel = "mistral-embed"

	// EmbeddingsPath 嵌入协议路径（端点变更时仅改此处）.
	// [EN] Embeddings protocol path (single source of truth).
	EmbeddingsPath = "/embeddings"
)
