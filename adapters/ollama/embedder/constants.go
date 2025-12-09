/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 22:06:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 22:06:00
 * @FilePath: \go-llmx\adapters\ollama\embedder\constants.go
 * @Description: Ollama 嵌入适配器常量 —— 默认端点/模型/协议路径
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollamaembed

// 默认端点与模型.
// [EN] Default endpoint and model.
const (
	// DefaultBaseURL Ollama 本地端点（远程/代理服务用 WithBaseURL/SetBaseURL 覆盖）.
	// [EN] Local Ollama endpoint (override for remote/proxy).
	DefaultBaseURL = "http://localhost:11434"

	// DefaultModel 默认嵌入模型（通用嵌入款）.
	// [EN] Default embedding model (general-purpose).
	DefaultModel = "nomic-embed-text"

	// EmbedPath 嵌入协议路径（原生 /api/embed；OpenAI 兼容层走 openai embedder）.
	// [EN] Embed protocol path (native /api/embed).
	EmbedPath = "/api/embed"
)

// wire 协议字面量.
// [EN] Wire protocol literals.
const (
	// errorTypeOllamaEmbed 协议错误体标记（错误体仅 message，无类型体系）.
	// [EN] Protocol error marker (message only, no type system).
	errorTypeOllamaEmbed = "ollama_embed_error"
)
