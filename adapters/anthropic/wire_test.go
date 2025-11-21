/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 22:05:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 22:05:00
 * @FilePath: \go-llmx\adapters\anthropic\wire_test.go
 * @Description: Anthropic 适配器编解码测试 —— 消息/内容块/图片源/工具调用与结果的双向
 * 转换（system 顶层、tool_result 块、base64/url 图片源、max_tokens 必填）.
 * mock 基建见 anthropic_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcanthropic

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateContent_ThinkingBlocks(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"content": [
				{"type": "thinking", "thinking": "先想一想"},
				{"type": "text", "text": "答案"}
			],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 1, "output_tokens": 2}
		}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	assert.Equal(t, "答案", resp.Choices[0].Text())
	assert.Equal(t, "先想一想", resp.Choices[0].Reasoning)
}

func TestGenerateContent_ToolUse(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"content": [
				{"type": "text", "text": "查询中"},
				{"type": "tool_use", "id": "tu_1", "name": "weather", "input": {"city": "北京"}}
			],
			"stop_reason": "tool_use",
			"usage": {"input_tokens": 1, "output_tokens": 2}
		}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("北京天气")})
	require.NoError(t, err)

	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "tu_1", calls[0].ID)
	assert.Equal(t, "weather", calls[0].Name)
	assert.JSONEq(t, `{"city":"北京"}`, calls[0].Arguments)
}

func TestGenerateContent_ToolResultEncoding(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "ok"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		llmx.User("q"),
		{Role: llmx.RoleAssistant, Content: []llmx.Part{llmx.ToolCallPart{ID: "tu_1", Name: "weather", Arguments: `{"city":"北京"}`}}},
		{Role: llmx.RoleTool, ToolCallID: "tu_1", Content: []llmx.Part{llmx.ToolResultPart{Result: `{"temp":25}`}}},
	})
	require.NoError(t, err)

	// 编码为 3 条消息：user 提问 / assistant(tool_use) / user(tool_result)
	msgs := m.body("messages").([]any)
	require.Len(t, msgs, 3)
	toolMsg := msgs[2].(map[string]any)
	assert.Equal(t, "user", toolMsg["role"])
	blocks := toolMsg["content"].([]any)
	require.Len(t, blocks, 1)
	block := blocks[0].(map[string]any)
	assert.Equal(t, "tool_result", block["type"])
	assert.Equal(t, "tu_1", block["tool_use_id"])
	assert.Equal(t, `{"temp":25}`, block["content"])
}

func TestGenerateContent_ToolResultError(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "ok"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		{Role: llmx.RoleTool, ToolCallID: "tu_1", Content: []llmx.Part{llmx.ToolResultPart{Error: "boom"}}},
	})
	require.NoError(t, err)

	msgs := m.body("messages").([]any)
	blocks := msgs[0].(map[string]any)["content"].([]any)
	block := blocks[0].(map[string]any)
	// 失败结果置 is_error 标记
	assert.Equal(t, true, block["is_error"])
}

func TestGenerateContent_ImageParts(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "看到了"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{{
		Role: llmx.RoleUser,
		Content: []llmx.Part{
			llmx.TextPart{Text: "看图"},
			llmx.ImagePart{Data: []byte("fake")},
			llmx.ImagePart{URL: "https://example.com/x.png"},
		},
	}})
	require.NoError(t, err)

	msgs := m.body("messages").([]any)
	content := msgs[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 3)

	// 文本块
	assert.Equal(t, "text", content[0].(map[string]any)["type"])
	// base64 内联图（缺省 MIME image/png）
	img1 := content[1].(map[string]any)["source"].(map[string]any)
	assert.Equal(t, "base64", img1["type"])
	assert.Equal(t, "image/png", img1["media_type"])
	// URL 图
	img2 := content[2].(map[string]any)["source"].(map[string]any)
	assert.Equal(t, "url", img2["type"])
	assert.Equal(t, "https://example.com/x.png", img2["url"])
}

func TestGenerateContent_EmptyContent(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
}

func TestGenerateContent_MaxTokensRequired(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "ok"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	require.NoError(t, err)
	// 协议必填：无配置走默认
	assert.Equal(t, float64(DefaultMaxTokens), m.body("max_tokens"))

	// 请求级覆盖
	_, err = c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")}, llmx.WithMaxTokens(512))
	require.NoError(t, err)
	assert.Equal(t, float64(512), m.body("max_tokens"))
}

func TestEncodeEmptyArgumentsFallback(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"content": [{"type": "text", "text": "ok"}], "stop_reason": "end_turn", "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		{Role: llmx.RoleAssistant, Content: []llmx.Part{llmx.ToolCallPart{ID: "x", Name: "noop"}}},
	})
	require.NoError(t, err)

	// 空参数兜底空对象（协议要求 Input 为 JSON 对象）
	msgs := m.body("messages").([]any)
	blocks := msgs[0].(map[string]any)["content"].([]any)
	input := blocks[0].(map[string]any)["input"]
	assert.Equal(t, map[string]any{}, input)
}

// TestParseArguments_EdgeCases 参数解析边界：空白串 / 非法 JSON / 合法 JSON.
// [EN] Argument parsing edges: blank / invalid JSON / valid JSON.
func TestParseArguments_EdgeCases(t *testing.T) {
	// 空白串 → 空对象
	assert.Equal(t, map[string]any{}, parseArguments("   "))
	// 非对象合法 JSON（数组）→ 解析失败兜底空对象
	assert.Equal(t, map[string]any{}, parseArguments(`[1,2]`))
	// 非法 JSON → 兜底空对象
	assert.Equal(t, map[string]any{}, parseArguments(`{broken`))
	// 合法对象 → 原样解析
	assert.Equal(t, map[string]any{"a": float64(1)}, parseArguments(`{"a":1}`))
}

// TestEncodeArguments_EdgeCases 参数编码边界：nil / null / 序列化失败 / 正常对象.
// [EN] Argument encoding edges: nil / null / marshal failure / normal object.
func TestEncodeArguments_EdgeCases(t *testing.T) {
	assert.Equal(t, emptyJSONObject, encodeArguments(nil))
	// JSON null 字面量（nil 指针序列化产物）→ 兜底空对象
	var nilMap *map[string]any
	assert.Equal(t, emptyJSONObject, encodeArguments(nilMap))
	// 序列化失败（channel 不支持 JSON）→ 兜底空对象
	assert.Equal(t, emptyJSONObject, encodeArguments(map[string]any{"ch": make(chan int)}))
	// 正常对象 → JSON 串
	assert.JSONEq(t, `{"a":1}`, encodeArguments(map[string]any{"a": 1}))
}
