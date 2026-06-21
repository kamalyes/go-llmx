/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 20:31:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-21 10:11:36
 * @FilePath: \go-llmx\adapters\anthropic\constants.go
 * @Description: Anthropic 适配器常量 —— 默认端点/模型/wire 协议字面量/SSE 事件名
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcanthropic

// 默认端点与模型.
// [EN] Default endpoint and model.
const (
	// DefaultBaseURL Anthropic 官方端点（兼容网关用 WithBaseURL/SetBaseURL 覆盖）.
	// [EN] Anthropic official endpoint (override via WithBaseURL/SetBaseURL).
	DefaultBaseURL = "https://api.anthropic.com"

	// DefaultModel 默认模型.
	// [EN] Default model.
	DefaultModel = "claude-sonnet-4-5"

	// MessagesPath Messages 协议路径（端点变更时仅改此处）.
	// [EN] Messages protocol path (single source of truth).
	MessagesPath = "/v1/messages"

	// DefaultMaxTokens 默认输出 token 上限（Anthropic 协议 max_tokens 必填）.
	// [EN] Default max output tokens (max_tokens is required by the protocol).
	DefaultMaxTokens = 4096
)

// 协议头字面量.
// [EN] Protocol header literals.
const (
	// headerAPIKey 认证头（Anthropic 专用密钥头，非 Bearer）.
	// [EN] Auth header (Anthropic-specific key header, not Bearer).
	headerAPIKey = "x-api-key"

	// headerVersion 协议版本头.
	// [EN] Protocol version header.
	headerVersion = "anthropic-version"

	// APIVersion 协议版本取值.
	// [EN] Protocol version value.
	APIVersion = "2023-06-01"
)

// wire 协议字面量（请求/响应 JSON 的固定取值）.
// [EN] Wire protocol literals (fixed JSON values).
const (
	// roleUser / roleAssistant 角色取值（system 走顶层参数，tool 走 user 消息内 tool_result 块）.
	// [EN] Role literals (system goes top-level; tool results live in user messages).
	roleUser      = "user"
	roleAssistant = "assistant"

	// blockTypeText 文本内容块类型.
	// [EN] Text content block type.
	blockTypeText = "text"

	// blockTypeImage 图片内容块类型.
	// [EN] Image content block type.
	blockTypeImage = "image"

	// blockTypeToolUse 工具调用块类型（assistant 侧）.
	// [EN] Tool call block type (assistant side).
	blockTypeToolUse = "tool_use"

	// blockTypeToolResult 工具结果块类型（user 侧回传）.
	// [EN] Tool result block type (user side).
	blockTypeToolResult = "tool_result"

	// blockTypeThinking 思考轨迹块类型（extended thinking）.
	// [EN] Thinking trace block type (extended thinking).
	blockTypeThinking = "thinking"

	// sourceTypeBase64 / sourceTypeURL 图片源类型.
	// [EN] Image source types.
	sourceTypeBase64 = "base64"
	sourceTypeURL    = "url"

	// stopReasonEndTurn 等结束原因取值（原样透传，保留 provider 语义）.
	// [EN] Stop reason literals (passed through to keep provider semantics).
	stopReasonEndTurn      = "end_turn"
	stopReasonMaxTokens    = "max_tokens"
	stopReasonStopSequence = "stop_sequence"
	stopReasonToolUse      = "tool_use"

	// errorTypeAnthropic 错误响应体的顶层类型标记.
	// [EN] Top-level type marker of error bodies.
	errorTypeAnthropic = "error"
)

// SSE 事件与增量字面量（流式协议）.
// [EN] SSE event and delta literals (streaming protocol).
const (
	// eventMessageStart 消息开始（携带初始 usage）.
	// [EN] Message start (carries initial usage).
	eventMessageStart = "message_start"

	// eventContentBlockStart 内容块开始（tool_use 块携带 id/name）.
	// [EN] Content block start (tool_use carries id/name).
	eventContentBlockStart = "content_block_start"

	// eventContentBlockDelta 内容块增量.
	// [EN] Content block delta.
	eventContentBlockDelta = "content_block_delta"

	// eventContentBlockStop 内容块结束.
	// [EN] Content block stop.
	eventContentBlockStop = "content_block_stop"

	// eventMessageDelta 消息级增量（结束原因 + 输出用量）.
	// [EN] Message-level delta (stop reason + output usage).
	eventMessageDelta = "message_delta"

	// eventMessageStop 消息结束.
	// [EN] Message stop.
	eventMessageStop = "message_stop"

	// eventPing 心跳.
	// [EN] Heartbeat.
	eventPing = "ping"

	// deltaTypeText 文本增量.
	// [EN] Text delta.
	deltaTypeText = "text_delta"

	// deltaTypeInputJSON 工具参数 JSON 增量.
	// [EN] Tool arguments JSON delta.
	deltaTypeInputJSON = "input_json_delta"

	// deltaTypeThinking 思考轨迹增量.
	// [EN] Thinking trace delta.
	deltaTypeThinking = "thinking_delta"
)

// wire 编解码字面量（encode/decode 内的固定取值）.
// [EN] Codec literals (fixed values inside encode/decode).
const (
	// thinkingEnabled 思考参数类型取值（wireThinking.Type 固定值）.
	// [EN] Thinking parameter type literal.
	thinkingEnabled = "enabled"

	// defaultImageMIME 内联图片缺省 MIME 类型.
	// [EN] Default MIME type for inline images.
	defaultImageMIME = "image/png"

	// emptyJSONObject 工具调用空参数兜底（协议要求 Input 为 JSON 对象）.
	// [EN] Empty-arguments fallback (protocol requires a JSON object).
	emptyJSONObject = "{}"

	// errorTypeHTTP 非 JSON 响应体回退时的错误类型标记.
	// [EN] Fallback error type for non-JSON bodies.
	errorTypeHTTP = "http_error"
)
