/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-06 21:19:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-06 21:38:57
 * @FilePath: \go-llmx\agent\agent_test.go
 * @Description: 自主体测试 —— FakeModel 驱动的全路径覆盖：
 * 直答/工具循环/一轮多工具并行/错误自愈/记忆回写/迭代上限/流式/选项透传
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/memory"
	"github.com/kamalyes/go-llmx/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoTool 回显工具（记录收到的参数）.
// [EN] Echo tool (records received arguments).
func echoTool(name string, seen *[]string) tool.Tool {
	return tool.Tool{
		Name:        name,
		Description: "回显参数",
		Parameters:  map[string]any{"type": "object"},
		Func: func(_ context.Context, args string) (string, error) {
			if seen != nil {
				*seen = append(*seen, args)
			}
			return "echo:" + args, nil
		},
	}
}

// textResponse 构造纯文本预设响应.
// [EN] Build a plain-text canned response.
func textResponse(text string) *llmx.Response {
	return &llmx.Response{
		Choices: []llmx.Choice{{Content: []llmx.Part{llmx.TextPart{Text: text}}, FinishReason: "stop"}},
		Usage:   llmx.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10},
	}
}

// toolCallResponse 构造含工具调用的预设响应.
// [EN] Build a canned response with tool calls.
func toolCallResponse(calls ...llmx.ToolCallPart) *llmx.Response {
	parts := make([]llmx.Part, len(calls))
	for i, c := range calls {
		parts[i] = c
	}
	return &llmx.Response{
		Choices: []llmx.Choice{{Content: parts, FinishReason: "tool_calls"}},
		Usage:   llmx.Usage{PromptTokens: 12, CompletionTokens: 8, TotalTokens: 20},
	}
}

// toolResultAt 取消息序列中第 i 个 tool 消息.
// [EN] Get the i-th tool message in a sequence.
func toolResultAt(msgs []llmx.Message, i int) llmx.Message {
	var seen int
	for _, m := range msgs {
		if m.Role == llmx.RoleTool {
			if seen == i {
				return m
			}
			seen++
		}
	}
	return llmx.Message{}
}

func TestRun_DirectAnswer(t *testing.T) {
	f := llmx.NewFakeModel("直接回答")

	res, err := New(f).Run(context.Background(), "你好")
	require.NoError(t, err)
	assert.Equal(t, "直接回答", res.Answer)
	assert.Empty(t, res.Steps)

	// NewFakeModel 未携带 Usage，直答路径无累积
	assert.Zero(t, res.Usage.TotalTokens)
	require.Len(t, f.Calls, 1)
}

func TestRun_ToolLoop(t *testing.T) {
	var seen []string
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "c1", Name: "echo", Arguments: `{"q":"天气"}`}),
		textResponse("北京 25 度"),
	}

	res, err := New(f).Tools(echoTool("echo", &seen)).Run(context.Background(), "北京天气")
	require.NoError(t, err)
	assert.Equal(t, "北京 25 度", res.Answer)
	assert.Equal(t, []string{`{"q":"天气"}`}, seen)

	// 轨迹：单步记录调用与观测
	require.Len(t, res.Steps, 1)
	assert.Equal(t, "echo", res.Steps[0].Call.Name)
	assert.Equal(t, `echo:{"q":"天气"}`, res.Steps[0].Observation)

	// 用量累积：工具轮 20 + 回答轮 10
	assert.Equal(t, 30, res.Usage.TotalTokens)

	// 第二轮消息结构：user → assistant(工具调用) → tool(结果)
	require.Len(t, f.Calls, 2)
	second := f.Calls[1]
	require.Len(t, second, 3)
	assert.Equal(t, llmx.RoleAssistant, second[1].Role)
	toolMsg := toolResultAt(second, 0)
	assert.Equal(t, llmx.RoleTool, toolMsg.Role)
	assert.Equal(t, "c1", toolMsg.ToolCallID)
}

func TestRun_ParallelTools(t *testing.T) {
	var mu sync.Mutex
	var order []string
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(
			llmx.ToolCallPart{ID: "c1", Name: "slow", Arguments: `{"i":1}`},
			llmx.ToolCallPart{ID: "c2", Name: "fast", Arguments: `{"i":2}`},
		),
		textResponse("两工具结果汇总"),
	}

	mk := func(name string) tool.Tool {
		return tool.Tool{
			Name: name, Description: "并行测试", Parameters: map[string]any{"type": "object"},
			Func: func(_ context.Context, args string) (string, error) {
				mu.Lock()
				order = append(order, name)
				mu.Unlock()
				return name + ":" + args, nil
			},
		}
	}

	res, err := New(f).Tools(mk("slow"), mk("fast")).Run(context.Background(), "并行")
	require.NoError(t, err)
	assert.Equal(t, "两工具结果汇总", res.Answer)

	// 两工具都执行且轨迹按模型调用序回填
	require.Len(t, res.Steps, 2)
	assert.Equal(t, "slow", res.Steps[0].Call.Name)
	assert.Equal(t, "fast", res.Steps[1].Call.Name)

	// 第二轮 tool 消息按调用序回填（c1 → c2），内容为 ToolResultPart
	second := f.Calls[1]
	first := toolResultAt(second, 0)
	secondT := toolResultAt(second, 1)
	assert.Equal(t, "c1", first.ToolCallID)
	assert.Equal(t, "c2", secondT.ToolCallID)
	fp, ok := first.Content[0].(llmx.ToolResultPart)
	require.True(t, ok)
	sp, ok := secondT.Content[0].(llmx.ToolResultPart)
	require.True(t, ok)
	assert.Equal(t, `slow:{"i":1}`, fp.Result)
	assert.Equal(t, `fast:{"i":2}`, sp.Result)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, order, 2)
}

func TestRun_ToolErrorRecovers(t *testing.T) {
	boom := tool.Tool{
		Name: "boom", Description: "总是失败", Parameters: map[string]any{"type": "object"},
		Func: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("工具内部故障")
		},
	}
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "c1", Name: "boom", Arguments: `{}`}),
		textResponse("换条路子解决了"),
	}

	res, err := New(f).Tools(boom).Run(context.Background(), "触发故障")
	require.NoError(t, err)
	assert.Equal(t, "换条路子解决了", res.Answer)

	// 错误以 Error 观测回传给模型
	require.Len(t, res.Steps, 1)
	assert.Equal(t, "工具内部故障", res.Steps[0].Observation)
	toolMsg := toolResultAt(f.Calls[1], 0)
	part, ok := toolMsg.Content[0].(llmx.ToolResultPart)
	require.True(t, ok)
	assert.Equal(t, "工具内部故障", part.Error)
}

func TestRun_UnknownToolRecovers(t *testing.T) {
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "c1", Name: "ghost", Arguments: `{}`}),
		textResponse("改用已有工具完成"),
	}

	res, err := New(f).Tools(echoTool("echo", nil)).Run(context.Background(), "调用不存在工具")
	require.NoError(t, err)
	assert.Equal(t, "改用已有工具完成", res.Answer)
	assert.Contains(t, res.Steps[0].Observation, `unknown tool "ghost"`)
}

func TestRun_MemoryWriteBack(t *testing.T) {
	mem := memory.NewBuffer()
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "c1", Name: "echo", Arguments: `{}`}),
		textResponse("最终答案"),
	}

	_, err := New(f).Tools(echoTool("echo", nil)).Memory(mem).Run(context.Background(), "第一问")
	require.NoError(t, err)

	// 记忆只落 user 与最终回答对，中间的 assistant(工具调用)/tool 轮不落
	msgs := mem.Messages()
	require.Len(t, msgs, 2)
	assert.Equal(t, llmx.RoleUser, msgs[0].Role)
	assert.Equal(t, "第一问", msgs[0].String())
	assert.Equal(t, llmx.RoleAssistant, msgs[1].Role)
	assert.Equal(t, "最终答案", msgs[1].String())

	// 第二问：记忆历史进入新一轮初始上下文（第三次模型调用：历史 2 条 + 本轮 user）
	f.Responses = []*llmx.Response{textResponse("第二答")}
	_, err = New(f).Tools(echoTool("echo", nil)).Memory(mem).Run(context.Background(), "第二问")
	require.NoError(t, err)
	require.Len(t, f.Calls, 3)
	third := f.Calls[2]
	require.Len(t, third, 3)
	assert.Equal(t, "第一问", third[0].String())
	assert.Equal(t, llmx.RoleAssistant, third[1].Role)
	assert.Equal(t, "第二问", third[2].String())
}

func TestRun_MaxIterations(t *testing.T) {
	// FakeModel 耗尽后复读最后一个响应 → 每轮都请求工具
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{
		toolCallResponse(llmx.ToolCallPart{ID: "c1", Name: "echo", Arguments: `{}`}),
	}

	res, err := New(f).Tools(echoTool("echo", nil)).MaxIterations(3).Run(context.Background(), "无限循环")
	require.ErrorIs(t, err, llmx.ErrMaxToolIterations)
	require.NotNil(t, res)
	assert.Len(t, res.Steps, 3)
	assert.Len(t, f.Calls, 3)
	assert.Empty(t, res.Answer)
}

func TestRun_Stream(t *testing.T) {
	// FakeModel 流式路径将预设响应逐字符吐出（纯文本场景验证最终回答逐帧透传）
	f := llmx.NewFakeModel("流式最终回答")

	var collected strings.Builder
	_, err := New(f).Tools(echoTool("echo", nil)).
		Stream(func(c *llmx.Chunk) error {
			collected.WriteString(c.Content)
			return nil
		}).
		Run(context.Background(), "流式")
	require.NoError(t, err)
	assert.Equal(t, "流式最终回答", collected.String())
}

func TestRun_SystemPromptAndOptions(t *testing.T) {
	f := llmx.NewFakeModel("收到")

	_, err := New(f).System("你是严谨助手").Run(context.Background(), "问", llmx.WithTemperature(0.1))
	require.NoError(t, err)

	require.Len(t, f.Calls, 1)
	msgs := f.Calls[0]
	require.Len(t, msgs, 2)
	assert.Equal(t, llmx.RoleSystem, msgs[0].Role)
	assert.Equal(t, "你是严谨助手", msgs[0].String())

	// 用户选项透传
	assert.Equal(t, 0.1, f.CallOptions[0].Temperature)
}

func TestRun_OptionsCarryToolDefs(t *testing.T) {
	f := &llmx.FakeModel{}
	f.Responses = []*llmx.Response{textResponse("答")}

	_, err := New(f).Tools(echoTool("echo", nil)).Run(context.Background(), "问")
	require.NoError(t, err)

	opts := f.CallOptions[0]
	require.Len(t, opts.Tools, 1)
	assert.Equal(t, "echo", opts.Tools[0].Name)
}

func TestRun_MissingModel(t *testing.T) {
	_, err := New(nil).Run(context.Background(), "问")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestRun_InvalidTool(t *testing.T) {
	f := llmx.NewFakeModel("x")
	broken := tool.Tool{Name: "broken", Description: "无执行函数"}

	_, err := New(f).Tools(broken).Run(context.Background(), "问")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestRun_ToolError(t *testing.T) {
	f := &llmx.FakeModel{Err: errors.New("网络中断")}

	res, err := New(f).Tools(echoTool("echo", nil)).Run(context.Background(), "问")
	require.Error(t, err)
	require.NotNil(t, res) // 失败附带部分结果便于排查
}
