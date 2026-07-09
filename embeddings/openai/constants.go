/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 22:11:00
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-09 21:38:16
 * @FilePath: \go-llmx\embeddings\openai\constants.go
 * @Description: OpenAI 兼容嵌入适配器常量 —— 默认端点/模型/协议路径
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcembed

// 默认端点与模型.
// [EN] Default endpoint and model.
const (
	// DefaultBaseURL OpenAI 官方端点（兼容服务用 WithBaseURL/SetBaseURL 覆盖）.
	// [EN] OpenAI official endpoint (override via WithBaseURL/SetBaseURL).
	DefaultBaseURL = "https://api.openai.com/v1"

	// DefaultModel 默认嵌入模型（通用性价比款）.
	// [EN] Default embedding model (cost-effective).
	DefaultModel = "text-embedding-3-small"

	// EmbeddingsPath 嵌入协议路径（端点变更时仅改此处）.
	// [EN] Embeddings protocol path (single source of truth).
	EmbeddingsPath = "/embeddings"
)
