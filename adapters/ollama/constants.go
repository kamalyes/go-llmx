/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 20:23:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 20:23:00
 * @FilePath: \go-llmx\adapters\ollama\constants.go
 * @Description: Ollama 适配器常量 —— 默认端点/模型/wire 协议字面量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollama

// 默认端点与模型.
// [EN] Default endpoint and model.
const (
	// DefaultBaseURL Ollama 本地端点（远程/代理服务用 WithBaseURL/SetBaseURL 覆盖）.
	// [EN] Local Ollama endpoint (override for remote/proxy).
	DefaultBaseURL = "http://localhost:11434"

	// DefaultModel 默认模型（通用对话款，Ollama 3B 级）.
	// [EN] Default model (general-purpose 3B class).
	DefaultModel = "llama3.2"

	// ChatPath 对话协议路径（端点变更时仅改此处）.
	// [EN] Chat protocol path (single source of truth).
	ChatPath = "/api/chat"
)

// wire 协议字面量（请求/响应 JSON 的固定取值）.
// [EN] Wire protocol literals (fixed JSON values).
const (
	// roleSystem / roleUser / roleAssistant / roleTool 角色取值.
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"

	// toolTypeFunction 工具类型（协议当前唯一取值）.
	// [EN] Tool type (the only value in the protocol).
	toolTypeFunction = "function"

	// formatJSON 强制 JSON 输出模式.
	// [EN] JSON forced output mode.
	formatJSON = "json"

	// errorTypeOllama 协议错误体标记（Ollama 错误体无类型体系，仅 message）.
	// [EN] Protocol error marker (Ollama errors carry message only).
	errorTypeOllama = "ollama_error"

	// toolCallIDPrefix 工具调用合成 ID 前缀（协议无调用 ID，按函数名合成）.
	// [EN] Synthesized tool call ID prefix (protocol has no call IDs).
	toolCallIDPrefix = "call_"
)

// wire 编解码字面量（encode/decode 内的固定取值）.
// [EN] Codec literals (fixed values inside encode/decode).
const (
	// emptyJSONObject 工具调用空参数兜底（协议要求 Arguments 为 JSON 对象）.
	// [EN] Empty-arguments fallback (protocol requires a JSON object).
	emptyJSONObject = "{}"

	// toolResultErrorPrefix 工具执行失败结果注入响应的前缀.
	// [EN] Prefix injected into failed tool results.
	toolResultErrorPrefix = "ERROR: "
)
