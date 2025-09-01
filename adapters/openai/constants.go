/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-01 20:17:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-01 20:17:00
 * @FilePath: \go-llmx\adapters\openai\constants.go
 * @Description: OpenAI 兼容适配器常量 —— 默认端点/模型/wire 协议字面量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcopenai

// 默认端点与模型.
// [EN] Default endpoint and model.
const (
	// DefaultBaseURL OpenAI 官方端点（兼容服务用 WithBaseURL/SetBaseURL 覆盖）.
	// [EN] OpenAI official endpoint (override via WithBaseURL/SetBaseURL).
	DefaultBaseURL = "https://api.openai.com/v1"

	// DefaultModel 默认模型（通用对话性价比款）.
	// [EN] Default model (general-purpose cost-effective).
	DefaultModel = "gpt-4o-mini"

	// ChatCompletionsPath 对话补全协议路径（端点变更时仅改此处）.
	// [EN] Chat completions protocol path (single source of truth).
	ChatCompletionsPath = "/chat/completions"
)

// wire 协议字面量（请求/响应 JSON 的固定取值）.
// [EN] Wire protocol literals (fixed JSON values).
const (
	// roleSystem / roleUser / roleAssistant / roleTool 角色取值.
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"

	// partTypeText 文本内容片段类型.
	// [EN] Text content part type.
	partTypeText = "text"

	// partTypeImageURL 图片内容片段类型.
	// [EN] Image content part type.
	partTypeImageURL = "image_url"

	// toolTypeFunction 工具类型（协议当前唯一取值）.
	// [EN] Tool type (the only value in the protocol).
	toolTypeFunction = "function"

	// responseFormatJSONObject JSON 强制输出模式.
	// [EN] JSON forced output mode.
	responseFormatJSONObject = "json_object"

	// errorTypeHTTP 非 JSON 响应体回退时的错误类型标记.
	// [EN] Fallback error type for non-JSON bodies.
	errorTypeHTTP = "http_error"
)

// wire 编解码字面量（encode/decode 内的固定取值）.
// [EN] Codec literals (fixed values inside encode/decode).
const (
	// defaultImageMIME 内联图片缺省 MIME 类型.
	// [EN] Default MIME type for inline images.
	defaultImageMIME = "image/png"

	// dataURLPrefix 内联图片 data URL 协议前缀.
	// [EN] Data URL protocol prefix for inline images.
	dataURLPrefix = "data:"

	// dataURLBase64Sep data URL 的 base64 分隔标记.
	// [EN] Base64 separator inside data URLs.
	dataURLBase64Sep = ";base64,"

	// emptyJSONObject 工具调用空参数兜底（协议要求 Arguments 为合法 JSON 串）.
	// [EN] Empty-arguments fallback (protocol requires valid JSON).
	emptyJSONObject = "{}"

	// toolResultErrorPrefix 工具执行失败结果注入响应的前缀.
	// [EN] Prefix injected into failed tool results.
	toolResultErrorPrefix = "ERROR: "
)
