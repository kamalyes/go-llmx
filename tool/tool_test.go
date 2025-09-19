/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-19 21:53:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-19 21:53:00
 * @FilePath: \go-llmx\tool\tool_test.go
 * @Description: 工具调用循环测试 —— FakeModel 驱动的全路径覆盖：
 * 单轮/多轮/工具失败回传/未注册工具/迭代上限/参数校验
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package tool

import (
	"context"
	"errors"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWeatherTool 测试用天气工具.
// [EN] Weather tool for tests.
func fakeWeatherTool(calls *[]string) Tool {
	return Tool{
		Name:        "weather",
		Description: "查询城市天气",
		Parameters:  map[string]any{"type": "object"},
		Func: func(_ context.Context, args string) (string, error) {
			if calls != nil {
				*calls = append(*calls, args)
			}
			return `{"temp":25}`, nil
		},
	}
}

// toolCallResponse 构造含工具调用的预设响应.
// [EN] Build a canned response with tool calls.
func toolCallResponse(calls ...llmx.ToolCallPart) *llmx.Response {
	return &llmx.Response{Choices: []llmx.Choice{{Content: []llmx.Part(callsToParts(calls)), FinishReason: "tool_calls"}}}
}

// textResponse 构造纯文本预设响应.
// [EN] Build a plain-text canned response.
func textResponse(text string) *llmx.Response {
	return &llmx.Response{Choices: []llmx.Choice{{Content: []llmx.Part{llmx.TextPart{Text: text}}, FinishReason: "stop"}}}
}

// callsToParts 工具调用转 Part 切片.
// [EN] Convert tool calls to parts.
func callsToParts(calls []llmx.ToolCallPart) []llmx.Part {
	parts := make([]llmx.Part, len(calls))
	for i, c := range calls {
		parts[i] = c
	}
	return parts
}

func TestRunToolLoop_SingleRound(t *testing.T) {
	var args []string
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "c1", Name: "weather", Arguments: `{"city":"北京"}`}),
		textResponse("北京 25 度"),
	}

	resp, err := RunToolLoop(context.Background(), f, []llmx.Message{llmx.User("北京天气")}, []Tool{fakeWeatherTool(&args)})
	require.NoError(t, err)
	assert.Equal(t, "北京 25 度", resp.Choices[0].Text())
	assert.Equal(t, []string{`{"city":"北京"}`}, args)

	// 第二轮历史：user + assistant(工具调用) + tool(结果)
	require.Len(t, f.Calls, 2)
	second := f.Calls[1]
	require.Len(t, second, 3)
	assert.Equal(t, llmx.RoleAssistant, second[1].Role)
	assert.Len(t, second[1].ToolCalls(), 1)
	assert.Equal(t, llmx.RoleTool, second[2].Role)
	assert.Equal(t, "c1", second[2].ToolCallID)
	assert.Equal(t, `{"temp":25}`, second[2].Content[0].(llmx.ToolResultPart).Result)

	// 请求选项携带工具定义
	assert.Len(t, f.CallOptions[0].Tools, 1)
	assert.Equal(t, "weather", f.CallOptions[0].Tools[0].Name)
}

func TestRunToolLoop_MultiRound(t *testing.T) {
	var n int
	tool := Tool{
		Name:        "calc",
		Description: "计算",
		Func: func(_ context.Context, _ string) (string, error) {
			n++
			return "1", nil
		},
	}
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "a", Name: "calc"}),
		toolCallResponse(llmx.ToolCallPart{ID: "b", Name: "calc"}),
		textResponse("完成"),
	}

	resp, err := RunToolLoop(context.Background(), f, []llmx.Message{llmx.User("q")}, []Tool{tool})
	require.NoError(t, err)
	assert.Equal(t, "完成", resp.Choices[0].Text())
	assert.Equal(t, 2, n)
	// 第三轮历史含两轮 assistant + tool 回填
	require.Len(t, f.Calls, 3)
	assert.Len(t, f.Calls[2], 5)
}

func TestRunToolLoop_NoToolsPassthrough(t *testing.T) {
	f := llmx.NewFakeModel("直通")
	resp, err := RunToolLoop(context.Background(), f, []llmx.Message{llmx.User("q")}, nil)
	require.NoError(t, err)
	assert.Equal(t, "直通", resp.Choices[0].Text())
	// 无工具时不追加 WithTools 选项
	assert.Len(t, f.CallOptions[0].Tools, 0)
}

func TestRunToolLoop_ToolErrorFeedsBack(t *testing.T) {
	tool := Tool{
		Name: "boom",
		Func: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("execution failed")
		},
	}
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "c1", Name: "boom"}),
		textResponse("工具坏了，直接回答"),
	}

	resp, err := RunToolLoop(context.Background(), f, []llmx.Message{llmx.User("q")}, []Tool{tool})
	require.NoError(t, err)
	assert.Equal(t, "工具坏了，直接回答", resp.Choices[0].Text())

	// 失败结果以 ToolResultPart.Error 回传
	second := f.Calls[1]
	assert.Equal(t, "execution failed", second[2].Content[0].(llmx.ToolResultPart).Error)
}

func TestRunToolLoop_UnknownTool(t *testing.T) {
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "x", Name: "ghost"}),
	}
	_, err := RunToolLoop(context.Background(), f, []llmx.Message{llmx.User("q")}, []Tool{fakeWeatherTool(nil)})
	assert.ErrorIs(t, err, llmx.ErrToolNotFound)
}

func TestRunToolLoop_MaxIterations(t *testing.T) {
	// 模型永远请求工具 → 达到上限
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "a", Name: "calc"}),
	}
	spin := Tool{Name: "calc", Func: func(_ context.Context, _ string) (string, error) { return "x", nil }}
	_, err := RunToolLoop(context.Background(), f, []llmx.Message{llmx.User("q")}, []Tool{spin})
	assert.ErrorIs(t, err, llmx.ErrMaxToolIterations)
	// 默认 8 轮
	assert.Equal(t, DefaultMaxToolIterations, f.CallCount())
}

func TestRunToolLoop_CustomMaxIterations(t *testing.T) {
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "a", Name: "calc"}),
	}
	spin := Tool{Name: "calc", Func: func(_ context.Context, _ string) (string, error) { return "x", nil }}
	_, err := RunToolLoop(context.Background(), f, []llmx.Message{llmx.User("q")}, []Tool{spin}, llmx.WithMaxToolIterations(3))
	assert.ErrorIs(t, err, llmx.ErrMaxToolIterations)
	assert.Equal(t, 3, f.CallCount())
}

func TestRunToolLoop_InvalidInputs(t *testing.T) {
	f := llmx.NewFakeModel("ok")
	ctx := context.Background()

	// nil 模型
	_, err := RunToolLoop(ctx, nil, []llmx.Message{llmx.User("q")}, nil)
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)

	// 工具缺名 / 缺执行函数
	_, err = RunToolLoop(ctx, f, []llmx.Message{llmx.User("q")}, []Tool{{Func: fakeWeatherTool(nil).Func}})
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
	_, err = RunToolLoop(ctx, f, []llmx.Message{llmx.User("q")}, []Tool{{Name: "x"}})
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestRunToolLoop_ModelError(t *testing.T) {
	f := &llmx.FakeModel{Err: llmx.ErrProviderUnavailable}
	tool := Tool{Name: "t", Func: func(_ context.Context, _ string) (string, error) { return "", nil }}
	_, err := RunToolLoop(context.Background(), f, []llmx.Message{llmx.User("q")}, []Tool{tool})
	assert.ErrorIs(t, err, llmx.ErrProviderUnavailable)
}

func TestRunToolLoop_EmptyChoices(t *testing.T) {
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{{Choices: nil}}
	tool := Tool{Name: "t", Func: func(_ context.Context, _ string) (string, error) { return "", nil }}
	_, err := RunToolLoop(context.Background(), f, []llmx.Message{llmx.User("q")}, []Tool{tool})
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
}

func TestToolDef(t *testing.T) {
	tool := fakeWeatherTool(nil)
	def := tool.Def()
	assert.Equal(t, "weather", def.Name)
	assert.Equal(t, "查询城市天气", def.Description)
	assert.NotNil(t, def.Parameters)
}
