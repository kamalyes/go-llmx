/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 20:11:26
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 20:25:31
 * @FilePath: \go-llmx\adapters\mistral\constants.go
 * @Description: Mistral 适配器常量 —— 默认端点/模型/协议字面量
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmistral

// 默认端点与模型.
// [EN] Default endpoint and model.
const (
	// DefaultBaseURL Mistral 官方端点（自建网关用 WithBaseURL 覆盖）.
	// [EN] Mistral official endpoint.
	DefaultBaseURL = "https://api.mistral.ai/v1"

	// DefaultModel 默认模型（通用对话款）.
	// [EN] Default model.
	DefaultModel = "mistral-small-latest"

	// ChatCompletionsPath 对话补全协议路径.
	// [EN] Chat completions protocol path.
	ChatCompletionsPath = "/chat/completions"
)

// wire 协议字面量.
// [EN] Wire protocol literals.
const (
	// roleSystem / roleUser / roleAssistant / roleTool 角色取值.
	// [EN] Roles.
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"

	// partTypeText 文本片段类型.
	// [EN] Text part type.
	partTypeText = "text"

	// toolTypeFunction 工具类型.
	// [EN] Tool type.
	toolTypeFunction = "function"

	// responseFormatJSONObject JSON 强制输出模式.
	// [EN] JSON forced output mode.
	responseFormatJSONObject = "json_object"

	// errorTypeInvalidRequest / errorTypeRateLimit Mistral 错误类型.
	// [EN] Mistral error types.
	errorTypeInvalidRequest = "invalid_request_error"
	errorTypeRateLimit      = "rate_limit_error"

	// emptyJSONObject 工具调用空参数兜底.
	// [EN] Empty-arguments fallback.
	emptyJSONObject = "{}"
)
