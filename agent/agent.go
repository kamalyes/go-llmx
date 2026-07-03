/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-03 21:07:52
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-03 21:26:38
 * @FilePath: \go-llmx\agent\agent.go
 * @Description: 自主体 —— 原生工具调用 ReAct 循环：模型 ↔ 工具多轮对话直至产出最终回答，
 * 可选系统提示 / 会话记忆 / 流式输出，替代 langchaingo agents 包的
 * Agent+Executor 双层装配与 MRKL 文本解析（现代模型原生 tool calling 直连）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package agent

import (
	"context"
	"fmt"
	"sync"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/memory"
	"github.com/kamalyes/go-llmx/tool"
)

// Step 单次工具执行记录（调用请求 + 观测输出）.
// [EN] One tool execution record (the call plus its observation).
type Step struct {
	// Call 模型发起的工具调用（名称/参数/调用 ID）.
	// [EN] The tool call issued by the model.
	Call llmx.ToolCallPart

	// Observation 工具输出，或未知工具/执行失败的错误描述.
	// [EN] Tool output, or the error for unknown tools / failed runs.
	Observation string
}

// Result 一次完整 agent 运行的产物.
// [EN] The outcome of one full agent run.
type Result struct {
	// Answer 最终回答文本（模型不再发起工具调用的那轮输出）.
	// [EN] The final answer (output of the round with no more tool calls).
	Answer string

	// Usage 多轮调用的累积 token 用量.
	// [EN] Token usage accumulated across rounds.
	Usage llmx.Usage

	// Steps 工具执行轨迹（按轮次与轮内调用序排列）.
	// [EN] Tool execution trajectory (by round, then call order).
	Steps []Step
}

// Agent 原生工具调用自主体（模型 + 工具 + 记忆的编排单元）.
// [EN] A native tool-calling agent (model + tools + memory orchestration unit).
//
// 循环语义：携带工具定义调用模型 → 无 ToolCalls 即最终回答返回；
// 有则回填 assistant 消息、执行工具、追加观测结果继续下一轮，
// 直至产出回答或达到迭代上限（ErrMaxToolIterations）。
// 工具执行失败与未注册工具名不中断循环，以错误观测回传由模型自愈
type Agent struct {
	// model 目标模型.
	// [EN] Target model.
	model llmx.Model

	// system 系统提示（可选，循环首轮前置）.
	// [EN] System prompt (optional, prepended to the loop).
	system string

	// tools 注册工具集（须在首次 Run 前配置完毕）.
	// [EN] Registered tools (must be fully configured before the first Run).
	tools []tool.Tool

	// mem 会话记忆（可选，只回写本轮 user 输入与最终回答对，中间步骤不落记忆）.
	// [EN] Conversation memory (optional; only the user turn and the final
	// answer are memorized, intermediate steps stay intra-turn).
	mem memory.Memory

	// maxIter 迭代上限（<=0 走 tool.DefaultMaxToolIterations）.
	// [EN] Iteration limit (<=0 falls back to tool.DefaultMaxToolIterations).
	maxIter int

	// stream 流式回调（可选，透传给模型；最终回答逐帧输出）.
	// [EN] Stream handler (optional, forwarded to the model).
	stream llmx.StreamHandler

	// once / defs / lookup 工具注册表预计算（首次 Run 构建一次，运行期零重复开销）.
	// [EN] Precomputed tool registry (built once on the first Run).
	once   sync.Once
	defs   []llmx.ToolDef
	lookup map[string]tool.Tool
}

// New 构造自主体并进入链式配置.
// [EN] Create an agent and enter chained configuration.
func New(model llmx.Model) *Agent {
	return &Agent{model: model}
}

// System 设置系统提示（循环首轮前置的 system 消息）.
// [EN] Set the system prompt (prepended as the first message).
func (a *Agent) System(prompt string) *Agent {
	a.system = prompt
	return a
}

// Tools 注册工具集（可多次调用追加；须在首次 Run 前完成）.
// [EN] Register tools (appendable; finish before the first Run).
func (a *Agent) Tools(tools ...tool.Tool) *Agent {
	a.tools = append(a.tools, tools...)
	return a
}

// Memory 绑定会话记忆（可选）.
// [EN] Bind conversation memory (optional).
func (a *Agent) Memory(m memory.Memory) *Agent {
	a.mem = m
	return a
}

// MaxIterations 设置迭代上限（<=0 走默认 8）.
// [EN] Set the iteration limit (<=0 for the default 8).
func (a *Agent) MaxIterations(n int) *Agent {
	a.maxIter = n
	return a
}

// Stream 绑定流式回调（可选，最终回答逐帧透传）.
// [EN] Bind a stream handler (optional; final answer streamed chunk by chunk).
func (a *Agent) Stream(h llmx.StreamHandler) *Agent {
	a.stream = h
	return a
}

// Run 执行自主体循环，返回最终回答与执行轨迹.
// [EN] Run the agent loop, returning the final answer and trajectory.
//
// 迭代上限耗尽仍无回答时返回 ErrMaxToolIterations（附带已累积的部分结果）；
// 模型调用失败同样附带部分结果便于排查轨迹；记忆仅在成功产出回答时回写
func (a *Agent) Run(ctx context.Context, input string, opts ...llmx.Option) (*Result, error) {
	if err := a.prepare(); err != nil {
		return nil, err
	}
	if a.model == nil {
		return nil, fmt.Errorf("%w: agent missing model", llmx.ErrInvalidRequest)
	}
	maxIter := a.maxIter
	if maxIter <= 0 {
		maxIter = tool.DefaultMaxToolIterations
	}

	// 初始上下文：系统提示 → 记忆历史 → 本轮输入
	messages := make([]llmx.Message, 0, len(a.defs)+2)
	if a.system != "" {
		messages = append(messages, llmx.System(a.system))
	}
	if a.mem != nil {
		messages = append(messages, a.mem.Messages()...)
	}
	messages = append(messages, llmx.User(input))

	// 工具定义统一追加在用户选项之后（用户选项优先，工具为 agent 自身职责）
	callOpts := append(append([]llmx.Option(nil), opts...), llmx.WithTools(a.defs...))

	var res Result
	for i := 0; i < maxIter; i++ {
		resp, err := a.generate(ctx, messages, callOpts)
		if err != nil {
			return &res, err
		}
		if len(resp.Choices) == 0 {
			return &res, llmx.ErrEmptyResponse
		}
		choice := resp.Choices[0]
		accumulateUsage(&res.Usage, resp.Usage)

		calls := choice.ToolCalls()
		if len(calls) == 0 {
			// 无工具调用即最终回答
			res.Answer = choice.Text()
			if a.mem != nil {
				a.mem.Add(llmx.User(input), llmx.Assistant(res.Answer))
			}
			return &res, nil
		}

		// assistant 回填（保留原始文本与工具调用 Part，供下轮上下文）
		messages = append(messages, llmx.Message{Role: llmx.RoleAssistant, Content: choice.Content})
		messages = append(messages, a.executeCalls(ctx, calls, &res)...)
	}
	return &res, llmx.ErrMaxToolIterations
}

// prepare 预计算工具注册表（首次调用构建，之后零开销直通）.
// [EN] Precompute the tool registry (built on first call, zero cost after).
func (a *Agent) prepare() error {
	var err error
	a.once.Do(func() {
		a.defs = make([]llmx.ToolDef, len(a.tools))
		a.lookup = make(map[string]tool.Tool, len(a.tools))
		for i, t := range a.tools {
			if t.Name == "" || t.Func == nil {
				err = fmt.Errorf("%w: tool %q missing name or func", llmx.ErrInvalidRequest, t.Name)
				return
			}
			a.defs[i] = t.Def()
			a.lookup[t.Name] = t
		}
	})
	return err
}

// generate 按是否绑定流式回调分派模型调用.
// [EN] Dispatch the model call by stream handler presence.
func (a *Agent) generate(ctx context.Context, messages []llmx.Message, opts []llmx.Option) (*llmx.Response, error) {
	if a.stream != nil {
		return a.model.StreamGenerateContent(ctx, messages, a.stream, opts...)
	}
	return a.model.GenerateContent(ctx, messages, opts...)
}

// executeCalls 执行一轮全部工具调用并返回观测消息（按调用序回填）.
// [EN] Execute all tool calls of one round, returning observation messages in order.
func (a *Agent) executeCalls(ctx context.Context, calls []llmx.ToolCallPart, res *Result) []llmx.Message {
	msgs := make([]llmx.Message, len(calls))
	for i, call := range calls {
		msg, step := a.executeCall(ctx, call)
		msgs[i] = msg
		res.Steps = append(res.Steps, step)
	}
	return msgs
}

// executeCall 执行单个工具调用（失败产出错误观测，不中断循环）.
// [EN] Execute one tool call (failures yield error observations, not aborts).
func (a *Agent) executeCall(ctx context.Context, call llmx.ToolCallPart) (llmx.Message, Step) {
	t, ok := a.lookup[call.Name]
	if !ok {
		obs := fmt.Sprintf("unknown tool %q", call.Name)
		return errorObservation(call, obs), Step{Call: call, Observation: obs}
	}
	out, err := t.Func(ctx, call.Arguments)
	if err != nil {
		obs := err.Error()
		return errorObservation(call, obs), Step{Call: call, Observation: obs}
	}
	return llmx.ToolResult(call.ID, out), Step{Call: call, Observation: out}
}

// errorObservation 构造错误形态的工具结果消息（Error 字段回传，模型可据此调整重试）.
// [EN] Build an error-shaped tool result message (the model may retry accordingly).
func errorObservation(call llmx.ToolCallPart, msg string) llmx.Message {
	return llmx.Message{
		Role:       llmx.RoleTool,
		Content:    []llmx.Part{llmx.ToolResultPart{Error: msg}},
		ToolCallID: call.ID,
		Name:       call.Name,
	}
}

// accumulateUsage 累积一轮用量.
// [EN] Accumulate one round of usage.
func accumulateUsage(dst *llmx.Usage, u llmx.Usage) {
	dst.PromptTokens += u.PromptTokens
	dst.CompletionTokens += u.CompletionTokens
	dst.TotalTokens += u.TotalTokens
}
