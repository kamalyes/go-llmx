/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-01 20:12:37
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-01 20:18:53
 * @FilePath: \go-llmx\adapters\googleai\constants.go
 * @Description: Google AI (Gemini) 适配器常量 —— 默认端点/模型/协议字面量
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgoogleai

// 默认端点与模型.
// [EN] Default endpoint and model.
const (
	// DefaultBaseURL Google AI 官方端点（Vertex AI 网关用 WithBaseURL 覆盖）.
	// [EN] Google AI official endpoint (override for Vertex AI gateway).
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

	// DefaultModel 默认模型（Flash 通用性价比款）.
	// [EN] Default model (Flash general-purpose).
	DefaultModel = "gemini-2.0-flash"
)

// wire 协议字面量（请求/响应 JSON 的固定取值）.
// [EN] Wire protocol literals.
const (
	// roleUser / roleModel 对话角色（Gemini 用 model 而非 assistant）.
	// [EN] Conversation roles (Gemini uses model instead of assistant).
	roleUser  = "user"
	roleModel = "model"

	// methodGenerateContent 非流式方法名（模型名拼路径，无法走 Base.Path）.
	// [EN] Non-streaming method (model name embedded in the path).
	methodGenerateContent = ":generateContent"

	// methodStreamContent 流式方法名（SSE 形态 alt=sse）.
	// [EN] Streaming method (SSE via alt=sse).
	methodStreamContent = ":streamGenerateContent"

	// sseQuery 流式 SSE 查询参数.
	// [EN] Streaming SSE query parameter.
	sseQuery = "?alt=sse"

	// modelsPathPrefix 模型路径前缀（.../models/{model}:method）.
	// [EN] Model path prefix.
	modelsPathPrefix = "/models/"
)

// 编解码字面量.
// [EN] Codec literals.
const (
	// mimeJSON 强制 JSON 输出（JSONMode 映射 responseMimeType）.
	// [EN] Forced JSON output (JSONMode mapped to responseMimeType).
	mimeJSON = "application/json"

	// keyAPIKey 认证头名（Gemini 专用头而非 Bearer）.
	// [EN] Auth header name (Gemini-specific, not Bearer).
	keyAPIKey = "x-goog-api-key"

	// emptyJSONObject 工具响应空载荷兜底（协议要求 response 为 JSON 对象）.
	// [EN] Empty tool-response fallback (protocol requires a JSON object).
	emptyJSONObject = "{}"

	// resultKey 工具响应结果的包装键.
	// [EN] Wrapper key for tool response results.
	resultKey = "result"

	// toolErrorKey 工具执行错误的包装键.
	// [EN] Wrapper key for tool execution errors.
	toolErrorKey = "error"

	// defaultImageMIME 内联图片缺省 MIME 类型.
	// [EN] Default MIME type for inline images.
	defaultImageMIME = "image/png"
)

// finishReason 结束原因映射字面量.
// [EN] Finish reason literals.
const (
	// finishStop 正常结束.
	// [EN] Natural stop.
	finishStop = "STOP"

	// finishMaxTokens token 上限截断.
	// [EN] Token limit truncation.
	finishMaxTokens = "MAX_TOKENS"

	// finishSafety 安全策略拦截.
	// [EN] Safety policy block.
	finishSafety = "SAFETY"

	// finishRecitation 复述拦截.
	// [EN] Recitation block.
	finishRecitation = "RECITATION"
)
