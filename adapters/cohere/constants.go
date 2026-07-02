/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 22:11:38
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 22:47:11
 * @FilePath: \go-llmx\adapters\cohere\constants.go
 * @Description: Cohere 适配器常量 —— 默认端点/模型/协议字面量
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccohere

// 默认端点与模型.
// [EN] Default endpoint and model.
const (
	// DefaultBaseURL Cohere 官方端点.
	// [EN] Cohere official endpoint.
	DefaultBaseURL = "https://api.cohere.com/v2"

	// DefaultModel 默认模型（Command R+ 通用款）.
	// [EN] Default model (Command R+).
	DefaultModel = "command-r-plus-08-2024"

	// ChatPath v2 对话协议路径.
	// [EN] v2 chat protocol path.
	ChatPath = "/chat"
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

	// partTypeText / partTypeImageURL / partTypeToolResult 片段类型.
	// [EN] Part types.
	partTypeText       = "text"
	partTypeImageURL   = "image_url"
	partTypeToolResult = "tool_result"

	// toolTypeFunction 工具类型.
	// [EN] Tool type.
	toolTypeFunction = "function"

	// responseFormatJSONObject JSON 强制输出模式.
	// [EN] JSON forced output mode.
	responseFormatJSONObject = "json_object"

	// emptyJSONObject 工具调用空参数兜底.
	// [EN] Empty-arguments fallback.
	emptyJSONObject = "{}"

	// toolResultErrorPrefix 工具执行失败结果注入响应的前缀.
	// [EN] Prefix injected into failed tool results.
	toolResultErrorPrefix = "ERROR: "
)

// finishReason 协议结束原因字面量.
// [EN] Finish reason literals.
const (
	// finishComplete 正常完成.
	// [EN] Natural completion.
	finishComplete = "COMPLETE"

	// finishMaxTokens token 上限截断.
	// [EN] Token limit truncation.
	finishMaxTokens = "MAX_TOKENS"

	// finishToolCalls 工具调用待执行.
	// [EN] Tool calls pending.
	finishToolCalls = "TOOL_CALLS"

	// finishStopSequence 停止序列命中.
	// [EN] Stop sequence hit.
	finishStopSequence = "STOP_SEQUENCE"
)

// streamEventType 流式事件类型.
// [EN] Streaming event types.
const (
	// eventContentStart / eventContentDelta / eventContentEnd 文本内容生命周期.
	// [EN] Text content lifecycle.
	eventContentStart = "content-start"
	eventContentDelta = "content-delta"
	eventContentEnd   = "content-end"

	// eventToolCallStart / eventToolCallDelta / eventToolCallEnd 工具调用生命周期.
	// [EN] Tool call lifecycle.
	eventToolCallStart = "tool-call-start"
	eventToolCallDelta = "tool-call-delta"
	eventToolCallEnd   = "tool-call-end"

	// eventMessageEnd 消息收口（finish_reason + usage）.
	// [EN] Message finalization.
	eventMessageEnd = "message-end"
)
