/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-28 20:15:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-28 20:15:00
 * @FilePath: \go-llmx\transport\constants.go
 * @Description: 传输层公共常量 —— HTTP 方法/内容类型/SSE 协议标记，适配器共享
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package transport

// HTTP 方法常量.
// [EN] HTTP method constants.
const (
	// MethodGet GET 请求.
	// [EN] GET request.
	MethodGet = "GET"

	// MethodPost POST 请求.
	// [EN] POST request.
	MethodPost = "POST"

	// MethodPut PUT 请求.
	// [EN] PUT request.
	MethodPut = "PUT"
)

// 常用 Content-Type 值.
// [EN] Common Content-Type values.
const (
	// ContentTypeJSON JSON 请求/响应体.
	// [EN] JSON request/response body.
	ContentTypeJSON = "application/json"

	// ContentTypeEvent SSE 流式响应.
	// [EN] SSE streaming response.
	ContentTypeEvent = "text/event-stream"

	// ContentTypeNDJSON NDJSON 流式响应（Ollama 等本地推理协议）.
	// [EN] NDJSON streaming response (local inference protocols).
	ContentTypeNDJSON = "application/x-ndjson"
)

// SSE 协议行前缀.
// [EN] SSE protocol line prefixes.
const (
	// SSEDataPrefix data 数据行前缀.
	// [EN] Prefix of data lines.
	SSEDataPrefix = "data:"

	// SSEEventPrefix event 事件名行前缀.
	// [EN] Prefix of event lines.
	SSEEventPrefix = "event:"
)

// 协议标记.
// [EN] Protocol markers.
const (
	// SSEDoneMarker OpenAI 兼容流的结束标记.
	// [EN] End-of-stream marker for OpenAI-compatible streams.
	SSEDoneMarker = "[DONE]"
)

// 默认传输配置.
// [EN] Default transport configuration.
const (
	// DefaultTimeout 默认请求超时（LLM 生成普遍慢于常规 API）.
	// [EN] Default request timeout (LLM generation is slower than typical APIs).
	DefaultTimeout = "120s"

	// MaxErrorBodySnippet 错误响应体截断上限（日志友好，1KB）.
	// [EN] Max error body snippet length (log friendly, 1KB).
	MaxErrorBodySnippet = 1 << 10
)
