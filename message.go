/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-15 21:18:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-15 21:18:00
 * @FilePath: \go-llmx\message.go
 * @Description: 对话消息体系 —— Role 枚举 + Message 值类型 + Part 多模态内容.
 * 值类型而非接口：砍掉 langchaingo 的 ChatMessage 接口 + 工厂函数复杂度
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package llmx

// Role 对话角色.
// [EN] Conversation role.
type Role string

const (
	// RoleSystem 系统指令（设定模型行为）.
	// [EN] System instruction that shapes model behavior.
	RoleSystem Role = "system"

	// RoleUser 用户输入.
	// [EN] User input.
	RoleUser Role = "user"

	// RoleAssistant 模型回复（含工具调用请求）.
	// [EN] Model response (may contain tool call requests).
	RoleAssistant Role = "assistant"

	// RoleTool 工具执行结果（回传给模型）.
	// [EN] Tool execution result (returned to the model).
	RoleTool Role = "tool"
)

// Message 单条对话消息.
// [EN] A single conversation message.
type Message struct {
	// Role 消息角色.
	Role Role

	// Content 多模态内容（文本/图片/工具调用），纯文本场景长度为 1.
	Content []Part

	// ToolCallID RoleTool 时对应的工具调用 ID（与 assistant 的 ToolCallPart.ID 对应）.
	// [EN] Tool call ID this result answers (matches ToolCallPart.ID).
	ToolCallID string

	// Name 工具名（RoleTool 时可选，便于模型区分）.
	Name string
}

// Text 快捷构造纯文本消息.
// [EN] Shorthand to build a plain-text message.
func Text(role Role, text string) Message {
	return Message{Role: role, Content: []Part{TextPart{Text: text}}}
}

// Part 多模态内容片段接口（具体形态见 TextPart / ImagePart / ToolCallPart）.
// [EN] Multimodal content part interface.
type Part interface {
	isPart()
}

// TextPart 文本内容.
// [EN] Text content.
type TextPart struct {
	Text string
}

func (TextPart) isPart() {}

// ImagePart 图片内容（URL 与 Base64 数据二选一）.
// [EN] Image content (either URL or Base64 data).
type ImagePart struct {
	// URL 图片地址（公网可访问或 provider 的文件上传地址）.
	// [EN] Image URL.
	URL string

	// MIMEType 图片类型（image/png 等；Data 非空时必填）.
	// [EN] Image MIME type, required when Data is set.
	MIMEType string

	// Base64 编码的图片数据（与 URL 二选一）.
	// [EN] Base64-encoded image data (alternative to URL).
	Data []byte
}

func (ImagePart) isPart() {}

// ToolCallPart 模型发起的工具调用请求（出现在 assistant 消息中）.
// [EN] Tool call request issued by the model (in assistant messages).
type ToolCallPart struct {
	// ID 本次调用 ID（结果回传时作为 Message.ToolCallID）.
	ID string

	// Name 工具名.
	Name string

	// Arguments JSON 编码的参数串（保持字符串原样传递）.
	// [EN] JSON-encoded argument string (kept as-is).
	Arguments string
}

func (ToolCallPart) isPart() {}

// ToolResultPart 工具执行结果（RoleTool 消息的内容形态）.
// [EN] Tool execution result (content shape of RoleTool messages).
type ToolResultPart struct {
	// Result 工具输出（纯文本或 JSON 字符串）.
	Result string

	// Error 工具执行错误信息（非空时模型可据此调整重试策略）.
	Error string
}

func (ToolResultPart) isPart() {}

// Messages 辅助函数：提取消息全部文本内容（多 Part 拼接）.
// [EN] Extract all text content of a message (parts joined).
func (m Message) String() string {
	out := ""
	for _, p := range m.Content {
		if t, ok := p.(TextPart); ok {
			out += t.Text
		}
	}
	return out
}

// ToolCalls 提取消息中的全部工具调用请求（无则返回 nil）.
// [EN] Extract all tool call requests in the message, nil if none.
func (m Message) ToolCalls() []ToolCallPart {
	var calls []ToolCallPart
	for _, p := range m.Content {
		if tc, ok := p.(ToolCallPart); ok {
			calls = append(calls, tc)
		}
	}
	return calls
}

// System 快捷构造 system 消息.
// [EN] Shorthand to build a system message.
func System(text string) Message { return Text(RoleSystem, text) }

// User 快捷构造 user 消息.
// [EN] Shorthand to build a user message.
func User(text string) Message { return Text(RoleUser, text) }

// Assistant 快捷构造 assistant 消息.
// [EN] Shorthand to build an assistant message.
func Assistant(text string) Message { return Text(RoleAssistant, text) }

// ToolResult 快捷构造工具结果消息.
// [EN] Shorthand to build a tool result message.
func ToolResult(toolCallID, result string) Message {
	return Message{
		Role:       RoleTool,
		Content:    []Part{ToolResultPart{Result: result}},
		ToolCallID: toolCallID,
	}
}
