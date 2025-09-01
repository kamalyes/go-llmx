/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-01 21:50:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-01 21:50:00
 * @FilePath: \go-llmx\adapters\openai\wire_test.go
 * @Description: OpenAI 适配器编解码测试 —— 消息/角色/图片/工具调用与结果的双向转换.
 * mock 基建见 openai_test.go；编排层/流式/错误映射见对应 _test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolRoundtrip_EncodeMessages(t *testing.T) {
	// 工具结果消息编码：RoleTool + ToolCallID + Result
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "done"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	msgs := []llmx.Message{
		llmx.User("天气"),
		{Role: llmx.RoleAssistant, Content: []llmx.Part{llmx.ToolCallPart{ID: "c1", Name: "weather", Arguments: `{}`}}},
		llmx.ToolResult("c1", `{"temp": 25}`),
	}
	_, err := c.GenerateContent(context.Background(), msgs)
	require.NoError(t, err)

	sent := m.body("messages").([]any)
	require.Len(t, sent, 3)

	// assistant 消息带 tool_calls 数组
	assistant := sent[1].(map[string]any)
	tcs := assistant["tool_calls"].([]any)
	require.Len(t, tcs, 1)
	tc := tcs[0].(map[string]any)
	assert.Equal(t, "c1", tc["id"])
	fn := tc["function"].(map[string]any)
	assert.Equal(t, "weather", fn["name"])

	// tool 消息带 tool_call_id + content（数组形态的 text part）
	toolMsg := sent[2].(map[string]any)
	assert.Equal(t, "tool", toolMsg["role"])
	assert.Equal(t, "c1", toolMsg["tool_call_id"])
	toolContent := toolMsg["content"].([]any)
	tp := toolContent[0].(map[string]any)
	assert.Equal(t, "text", tp["type"])
	assert.Equal(t, `{"temp": 25}`, tp["text"])
}

func TestNonStreamToolCalls(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"choices": [{
				"message": {
					"role": "assistant", "content": null,
					"tool_calls": [{"id": "call_9", "type": "function", "function": {"name": "search", "arguments": "{\"q\":\"go\"}"}}]
				},
				"finish_reason": "tool_calls"
			}]
		}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("搜")})
	require.NoError(t, err)

	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "search", calls[0].Name)
	assert.Equal(t, `{"q":"go"}`, calls[0].Arguments)
}

func TestReasoningContent(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"choices": [{
				"message": {"role": "assistant", "content": "答", "reasoning_content": "思考过程"},
				"finish_reason": "stop"
			}]
		}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	assert.Equal(t, "思考过程", resp.Choices[0].Reasoning)
	assert.Equal(t, "答", resp.Choices[0].Text())
}

func TestMultimodalImageMessage(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "图里有猫"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		{
			Role: llmx.RoleUser,
			Content: []llmx.Part{
				llmx.TextPart{Text: "这是什么"},
				llmx.ImagePart{URL: "https://example.com/cat.png"},
			},
		},
	})
	require.NoError(t, err)

	// 多模态请求 content 为数组形态
	msgs := m.body("messages").([]any)
	first := msgs[0].(map[string]any)
	contentArr, ok := first["content"].([]any)
	require.True(t, ok, "多模态 content 应为数组")
	require.Len(t, contentArr, 2)
	imgPart := contentArr[1].(map[string]any)
	assert.Equal(t, "image_url", imgPart["type"])
	imgURL := imgPart["image_url"].(map[string]any)
	assert.Equal(t, "https://example.com/cat.png", imgURL["url"])
}

func TestEncodeImageDataURL(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		{
			Role: llmx.RoleUser,
			Content: []llmx.Part{
				// Data 非空 + 缺省 MIME → data:image/png;base64, 前缀
				llmx.ImagePart{Data: []byte("fakepng")},
			},
		},
	})
	require.NoError(t, err)

	msgs := m.body("messages").([]any)
	first := msgs[0].(map[string]any)
	content := first["content"].([]any)
	img := content[0].(map[string]any)["image_url"].(map[string]any)
	assert.Equal(t, "data:image/png;base64,ZmFrZXBuZw==", img["url"])
}

func TestEncodeEmptyArgumentsFallback(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		{Role: llmx.RoleAssistant, Content: []llmx.Part{llmx.ToolCallPart{ID: "x", Name: "noop"}}},
	})
	require.NoError(t, err)

	// 空 Arguments 兜底为 {}
	msgs := m.body("messages").([]any)
	tcs := msgs[0].(map[string]any)["tool_calls"].([]any)
	assert.Equal(t, "{}", tcs[0].(map[string]any)["function"].(map[string]any)["arguments"])
}

func TestEncodeRoleDefault(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		{Role: llmx.Role("custom"), Content: []llmx.Part{llmx.TextPart{Text: "q"}}},
	})
	require.NoError(t, err)

	msgs := m.body("messages").([]any)
	assert.Equal(t, "user", msgs[0].(map[string]any)["role"])
}

func TestEncodeRoleSystem(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		llmx.System("你是助手"),
		llmx.User("q"),
	})
	require.NoError(t, err)

	msgs := m.body("messages").([]any)
	assert.Equal(t, "system", msgs[0].(map[string]any)["role"])
	assert.Equal(t, "你是助手", msgs[0].(map[string]any)["content"])
}

func TestEncodeToolResultWithError(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		llmx.System("s"),
		{Role: llmx.RoleTool, ToolCallID: "c1", Content: []llmx.Part{llmx.ToolResultPart{Error: "boom"}}},
	})
	require.NoError(t, err)

	msgs := m.body("messages").([]any)
	toolContent := msgs[1].(map[string]any)["content"].([]any)
	tp := toolContent[0].(map[string]any)
	// 失败结果注入 ERROR: 前缀
	assert.Equal(t, "ERROR: boom", tp["text"])
}

func TestDecodePartsArrayContent(t *testing.T) {
	// 非流式响应 content 为数组形态（多模态回显场景）
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": [{"type": "text", "text": "分段一"}, {"type": "text", "text": "分段二"}]}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	assert.Equal(t, "分段一分段二", resp.Choices[0].Text())
}

func TestDecodePartsTolerance(t *testing.T) {
	// 数组内含未知类型/空文本/非对象元素 → 容忍跳过
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": [{"type": "image_url", "image_url": {"url": "x"}}, {"type": "text", "text": ""}, "raw-string", {"type": "text", "text": "有效"}]}, "finish_reason": "stop"}]}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	assert.Equal(t, "有效", resp.Choices[0].Text())
}
