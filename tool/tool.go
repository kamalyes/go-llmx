/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-19 21:12:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-19 21:23:00
 * @FilePath: \go-llmx\tool\tool.go
 * @Description: 工具调用体系 —— Tool 定义 + RunToolLoop 循环执行器.
 * ReAct 简化版：模型请求工具 → 执行 → 结果回传 → 直至产出最终回答，
 * 替代 langchaingo agents 包的 MRKL/Executor 装配复杂度
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package tool

import (
	"context"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
)

// ToolFunc 工具执行函数（arguments 为模型产出的 JSON 参数串，原样透传）.
// [EN] Tool executor (arguments is the model-produced JSON string, passed as-is).
type ToolFunc func(ctx context.Context, arguments string) (string, error)

// Tool 带执行函数的完整工具（ToolDef 为其暴露给模型的定义子集）.
// [EN] A complete tool with its executor (ToolDef is its model-facing subset).
type Tool struct {
	// Name 工具名（模型调用时引用，注册列表内须唯一）.
	// [EN] Tool name referenced by the model (must be unique in the registry).
	Name string

	// Description 功能描述（模型据此决定是否调用）.
	// [EN] Description (the model decides whether to call based on this).
	Description string

	// Parameters 参数 JSON Schema（map / SchemaOf 产物）.
	// [EN] Parameter JSON Schema (a map or a SchemaOf product).
	Parameters any

	// Func 执行函数.
	// [EN] Executor.
	Func ToolFunc
}

// Def 返回暴露给模型的工具定义.
// [EN] Return the model-facing tool definition.
func (t Tool) Def() llmx.ToolDef {
	return llmx.ToolDef{Name: t.Name, Description: t.Description, Parameters: t.Parameters}
}

// DefaultMaxToolIterations 工具循环默认上限（防失控自旋）.
// [EN] Default tool loop limit (guards against runaway loops).
const DefaultMaxToolIterations = 6

// RunToolLoop 执行工具调用循环直至模型产出最终回答.
// [EN] Run the tool call loop until the model yields a final answer.
//
// 每轮：携带工具定义调用模型 → 无 ToolCalls 即为最终回答返回；
// 否则回填 assistant 消息、执行全部工具、追加结果继续；
// 达到 MaxToolIterations 上限仍无最终回答返回 ErrMaxToolIterations
func RunToolLoop(ctx context.Context, m llmx.Model, messages []llmx.Message, tools []Tool, opts ...llmx.Option) (*llmx.Response, error) {
	if m == nil {
		return nil, fmt.Errorf("%w: model is nil", llmx.ErrInvalidRequest)
	}
	for _, t := range tools {
		if t.Name == "" || t.Func == nil {
			return nil, fmt.Errorf("%w: tool %q missing name or func", llmx.ErrInvalidRequest, t.Name)
		}
	}
	if len(tools) == 0 {
		return m.GenerateContent(ctx, messages, opts...)
	}

	o := llmx.Apply(opts...)
	maxIter := o.MaxToolIterations
	if maxIter <= 0 {
		maxIter = DefaultMaxToolIterations
	}

	defs := make([]llmx.ToolDef, len(tools))
	lookup := make(map[string]Tool, len(tools))
	for i, t := range tools {
		defs[i] = t.Def()
		lookup[t.Name] = t
	}

	history := append([]llmx.Message(nil), messages...)
	callOpts := append(append([]llmx.Option(nil), opts...), llmx.WithTools(defs...))

	for i := 0; i < maxIter; i++ {
		resp, err := m.GenerateContent(ctx, history, callOpts...)
		if err != nil {
			return nil, err
		}
		if len(resp.Choices) == 0 {
			return nil, llmx.ErrEmptyResponse
		}
		calls := resp.Choices[0].ToolCalls()
		if len(calls) == 0 {
			return resp, nil
		}

		// assistant 消息回填（保留原始文本与工具调用 Part，供下轮上下文）
		history = append(history, llmx.Message{Role: llmx.RoleAssistant, Content: resp.Choices[0].Content})

		for _, call := range calls {
			tool, ok := lookup[call.Name]
			if !ok {
				return nil, fmt.Errorf("%w: %s", llmx.ErrToolNotFound, call.Name)
			}
			result, err := tool.Func(ctx, call.Arguments)
			if err != nil {
				// 执行失败回传错误信息，模型可据此调整重试策略
				history = append(history, llmx.Message{
					Role:       llmx.RoleTool,
					ToolCallID: call.ID,
					Name:       call.Name,
					Content:    []llmx.Part{llmx.ToolResultPart{Error: err.Error()}},
				})
				continue
			}
			history = append(history, llmx.ToolResult(call.ID, result))
		}
	}
	return nil, llmx.ErrMaxToolIterations
}
