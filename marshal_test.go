/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-28 21:09:03
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-28 21:15:29
 * @FilePath: \go-llmx\marshal_test.go
 * @Description: 消息序列化测试 —— 四种 Part 无损往返/畸形输入防护.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalMessage_TextRoundTrip(t *testing.T) {
	orig := User("你好，世界")

	data, err := MarshalMessage(orig)
	require.NoError(t, err)

	back, err := UnmarshalMessage(data)
	require.NoError(t, err)
	assert.Equal(t, orig.Role, back.Role)
	assert.Equal(t, orig.String(), back.String())
}

func TestMarshalMessage_AllPartKindsRoundTrip(t *testing.T) {
	orig := Message{
		Role: RoleAssistant,
		Content: []Part{
			TextPart{Text: "let me use a tool"},
			ToolCallPart{ID: "call_1", Name: "search", Arguments: `{"q":"go"}`},
			ImagePart{URL: "https://example.com/a.png"},
			ImagePart{MIMEType: "image/jpeg", Data: []byte("raw-bytes")},
			ToolResultPart{Result: `{"hits":3}`, Error: ""},
			ToolResultPart{Result: "", Error: "tool failed"},
		},
	}

	data, err := MarshalMessage(orig)
	require.NoError(t, err)

	back, err := UnmarshalMessage(data)
	require.NoError(t, err)
	require.Len(t, back.Content, len(orig.Content))

	// 逐 Part 断言类型与内容还原
	assert.Equal(t, TextPart{Text: "let me use a tool"}, back.Content[0])
	assert.Equal(t, ToolCallPart{ID: "call_1", Name: "search", Arguments: `{"q":"go"}`}, back.Content[1])
	assert.Equal(t, ImagePart{URL: "https://example.com/a.png"}, back.Content[2])
	assert.Equal(t, ImagePart{MIMEType: "image/jpeg", Data: []byte("raw-bytes")}, back.Content[3])
	assert.Equal(t, ToolResultPart{Result: `{"hits":3}`}, back.Content[4])
	assert.Equal(t, ToolResultPart{Error: "tool failed"}, back.Content[5])
}

func TestMarshalMessage_EmptyContent(t *testing.T) {
	data, err := MarshalMessage(Message{Role: RoleSystem})
	require.NoError(t, err)

	back, err := UnmarshalMessage(data)
	require.NoError(t, err)
	assert.Empty(t, back.Content)
	assert.Equal(t, RoleSystem, back.Role)
}

func TestUnmarshalMessage_InvalidInput(t *testing.T) {
	// 非法 JSON
	_, err := UnmarshalMessage([]byte(`{not json`))
	require.Error(t, err)

	// 未知 Part 类型
	_, err = UnmarshalMessage([]byte(`{"role":"user","content":[{"type":"mystery"}]}`))
	require.ErrorIs(t, err, ErrInvalidRequest)
}

func TestMarshalMessage_JSONShape(t *testing.T) {
	data, err := MarshalMessage(User("hi"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"role":"user"`)
	assert.Contains(t, string(data), `"type":"text"`)
	assert.Contains(t, string(data), `"text":"hi"`)
}
