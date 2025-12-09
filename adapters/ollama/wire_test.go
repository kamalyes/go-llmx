/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 21:57:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 21:57:00
 * @FilePath: \go-llmx\adapters\ollama\wire_test.go
 * @Description: Ollama 适配器编解码测试 —— 消息/图片 base64/工具调用与结果的
 * 双向转换（工具参数对象形态、合成调用 ID、role:tool 结果消息、空响应）.
 * mock 基建见 ollama_test.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcollama

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateContent_ToolUse(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"message": {
				"role": "assistant",
				"content": "查询中",
				"tool_calls": [{"function": {"name": "weather", "arguments": {"city": "北京"}}}]
			},
			"done": true,
			"done_reason": "stop"
		}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	resp, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("北京天气")})
	require.NoError(t, err)

	calls := resp.Choices[0].ToolCalls()
	require.Len(t, calls, 1)
	// 协议无调用 ID → 按函数名合成
	assert.Equal(t, "call_weather", calls[0].ID)
	assert.Equal(t, "weather", calls[0].Name)
	// 协议差异：参数为 JSON 对象 → 解码为串
	assert.JSONEq(t, `{"city":"北京"}`, calls[0].Arguments)
}

func TestGenerateContent_ToolResultEncoding(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": "ok"}, "done": true, "done_reason": "stop"}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		llmx.User("q"),
		{Role: llmx.RoleAssistant, Content: []llmx.Part{llmx.ToolCallPart{ID: "call_weather", Name: "weather", Arguments: `{"city":"北京"}`}}},
		llmx.ToolResult("call_weather", `{"temp":25}`),
	})
	require.NoError(t, err)

	// 编码为 3 条消息：user / assistant(tool_calls) / tool(结果文本)
	msgs := m.body("messages").([]any)
	require.Len(t, msgs, 3)

	// assistant 工具调用：参数字符串解析为对象
	asst := msgs[1].(map[string]any)
	assert.Equal(t, "assistant", asst["role"])
	tcs := asst["tool_calls"].([]any)
	fn := tcs[0].(map[string]any)["function"].(map[string]any)
	assert.Equal(t, "weather", fn["name"])
	assert.Equal(t, map[string]any{"city": "北京"}, fn["arguments"])

	// 工具结果：协议差异为 role:tool 的文本消息（无 tool_call_id 字段）
	toolMsg := msgs[2].(map[string]any)
	assert.Equal(t, "tool", toolMsg["role"])
	assert.Equal(t, `{"temp":25}`, toolMsg["content"])
}

func TestGenerateContent_ImageParts(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": "看到了"}, "done": true, "done_reason": "stop"}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{{
		Role: llmx.RoleUser,
		Content: []llmx.Part{
			llmx.TextPart{Text: "看图"},
			llmx.ImagePart{Data: []byte("fake")},
			// 协议仅支持 base64 内联，URL 图片静默忽略
			llmx.ImagePart{URL: "https://example.com/x.png"},
		},
	}})
	require.NoError(t, err)

	msgs := m.body("messages").([]any)
	msg := msgs[0].(map[string]any)
	assert.Equal(t, "看图", msg["content"])

	// 内联图走 images base64 数组；URL 图片不产生条目
	images := msg["images"].([]any)
	require.Len(t, images, 1)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("fake")), images[0])
}

func TestGenerateContent_EmptyContent(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": ""}, "done": true, "done_reason": "stop"}`)
	})
	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{llmx.User("q")})
	assert.ErrorIs(t, err, llmx.ErrEmptyResponse)
}

// TestParseArguments_EdgeCases 参数解析边界：空白串 / 非法 JSON / 非对象 / 正常对象.
// [EN] Argument parsing edges: blank / invalid JSON / non-object / normal.
func TestParseArguments_EdgeCases(t *testing.T) {
	// 空串 → 空对象
	assert.Equal(t, map[string]any{}, parseArguments(""))
	// 空白串 → 空对象
	assert.Equal(t, map[string]any{}, parseArguments("   "))
	// 非法 JSON → 空对象
	assert.Equal(t, map[string]any{}, parseArguments(`{broken`))
	// 非对象合法 JSON（null）→ 空对象
	assert.Equal(t, map[string]any{}, parseArguments(`null`))
	// 正常对象 → 原样解析
	assert.Equal(t, map[string]any{"a": float64(1)}, parseArguments(`{"a":1}`))
}

// TestArgumentsToJSON_EdgeCases 参数编码边界：nil / 序列化失败 / 正常对象.
// [EN] Argument encoding edges: nil / marshal failure / normal object.
func TestArgumentsToJSON_EdgeCases(t *testing.T) {
	// nil → 空对象
	assert.Equal(t, emptyJSONObject, argumentsToJSON(nil))
	// 序列化失败（channel 不支持 JSON）→ 兜底空对象
	assert.Equal(t, emptyJSONObject, argumentsToJSON(make(chan int)))
	// 正常对象 → JSON 串
	assert.JSONEq(t, `{"a":1}`, argumentsToJSON(map[string]any{"a": 1}))
}

// TestEncodeToolResultWithText 文本与工具结果混合内容的换行拼接.
// [EN] Mixed text and tool result content joined by newline.
func TestEncodeToolResultWithText(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": "ok"}, "done": true, "done_reason": "stop"}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		{Role: llmx.RoleTool, ToolCallID: "call_x", Content: []llmx.Part{
			llmx.TextPart{Text: "前置说明"},
			llmx.ToolResultPart{Result: `{"ok":true}`},
		}},
	})
	require.NoError(t, err)

	msgs := m.body("messages").([]any)
	toolMsg := msgs[0].(map[string]any)
	assert.Equal(t, "前置说明\n"+`{"ok":true}`, toolMsg["content"])
}

// TestEncodeRole 角色映射与未知角色兜底.
// [EN] Role mapping and unknown-role fallback.
func TestEncodeRole(t *testing.T) {
	assert.Equal(t, roleSystem, encodeRole(llmx.RoleSystem))
	assert.Equal(t, roleUser, encodeRole(llmx.RoleUser))
	assert.Equal(t, roleAssistant, encodeRole(llmx.RoleAssistant))
	assert.Equal(t, roleTool, encodeRole(llmx.RoleTool))
	assert.Equal(t, roleUser, encodeRole(llmx.Role("weird")))
}

// TestEncodeToolResultError 工具失败结果注入 ERROR 前缀.
// [EN] Failed tool results get the ERROR prefix.
func TestEncodeToolResultError(t *testing.T) {
	m := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"message": {"role": "assistant", "content": "ok"}, "done": true, "done_reason": "stop"}`)
	})

	c := New("k", WithBaseURL(m.srv.URL))
	_, err := c.GenerateContent(context.Background(), []llmx.Message{
		{Role: llmx.RoleTool, ToolCallID: "call_x", Content: []llmx.Part{llmx.ToolResultPart{Error: "boom"}}},
	})
	require.NoError(t, err)

	msgs := m.body("messages").([]any)
	toolMsg := msgs[0].(map[string]any)
	assert.Equal(t, toolResultErrorPrefix+"boom", toolMsg["content"])
}
