/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-15 21:52:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-15 21:52:00
 * @FilePath: \go-llmx\response.go
 * @Description: 响应体系 —— Response/Choice/Usage + 流式 Chunk.
 * token 用量来自 API 返回的 Usage（不引入 tiktoken 本地计算）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package llmx

// Response 模型生成结果.
// [EN] Model generation result.
type Response struct {
	// Choices 候选回答（n>1 时多个，常规 n=1）.
	Choices []Choice

	// Usage token 用量统计.
	Usage Usage

	// Model 实际使用的模型名（可能与请求名不同，如自动路由）.
	Model string
}

// Choice 单个候选回答.
// [EN] A single candidate response.
type Choice struct {
	// Content 回答内容（文本/图片/工具调用的 Part 集合）.
	Content []Part

	// Reasoning 推理模型的思考轨迹（deepseek-reasoner / o1 等；无则空串）.
	// [EN] Reasoning trace for thinking models, empty if none.
	Reasoning string

	// FinishReason 结束原因（stop / length / tool_calls / content_filter）.
	FinishReason string

	// StopSequence 触发停止的序列（stop 参数命中时）.
	StopSequence string
}

// Text 提取回答文本（多 Part 拼接，与 Message.String 同构）.
// [EN] Extract response text (parts joined).
func (c Choice) Text() string {
	out := ""
	for _, p := range c.Content {
		if t, ok := p.(TextPart); ok {
			out += t.Text
		}
	}
	return out
}

// ToolCalls 提取回答中的工具调用请求，无则返回 nil.
// [EN] Extract tool call requests in the choice, nil if none.
func (c Choice) ToolCalls() []ToolCallPart {
	var calls []ToolCallPart
	for _, p := range c.Content {
		if tc, ok := p.(ToolCallPart); ok {
			calls = append(calls, tc)
		}
	}
	return calls
}

// Usage token 用量统计.
// [EN] Token usage statistics.
type Usage struct {
	// PromptTokens 输入 token 数.
	PromptTokens int

	// CompletionTokens 输出 token 数.
	CompletionTokens int

	// TotalTokens 总数（prompt + completion）.
	TotalTokens int
}

// Chunk 流式生成的一个增量片段.
// [EN] An incremental piece of a streamed response.
type Chunk struct {
	// Content 本次增量文本（空串 = 纯控制帧）.
	Content string

	// Reasoning 思考轨迹增量（reasoner 类模型；常规模型恒为空）.
	// [EN] Reasoning trace delta (reasoner models only).
	Reasoning string

	// ToolCallDelta 工具调用增量（Arguments 逐段拼接）.
	ToolCallDelta *ToolCallDelta

	// FinishReason 收到即表示流结束（stop / length / tool_calls）.
	FinishReason string

	// Usage 用量（多数 provider 在最后一帧携带，未携带为零值）.
	Usage Usage
}

// ToolCallDelta 工具调用的流式增量片段.
// [EN] Streaming delta of a tool call.
type ToolCallDelta struct {
	// Index 本次增量归属的工具调用序号（一轮可发起多个工具）.
	Index int

	// ID 调用 ID（首帧携带，后续帧为空）.
	ID string

	// Name 工具名（首帧携带，后续帧为空）.
	Name string

	// Arguments 参数 JSON 的增量片段（各帧拼接成完整串）.
	Arguments string
}

// StreamHandler 流式回调：每个 Chunk 调用一次.
// 返回 ErrStopStream 可提前终止（适配器立即断流并返回 ErrStreamClosed）.
// [EN] Stream callback invoked per chunk; return ErrStopStream to terminate early.
type StreamHandler func(chunk *Chunk) error

// StreamCollector 收集全部流式文本的便捷 handler（goroutine 安全由调用方保证）.
// [EN] Convenience handler that collects all streamed text.
func StreamCollector(buf *string) StreamHandler {
	return func(c *Chunk) error {
		*buf += c.Content
		return nil
	}
}
