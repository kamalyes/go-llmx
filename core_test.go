/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-15 22:08:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-15 22:08:00
 * @FilePath: \go-llmx\core_test.go
 * @Description: 核心抽象单测 —— 消息/选项/FakeModel/错误哨兵
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageHelpers(t *testing.T) {
	m := Message{
		Role: RoleAssistant,
		Content: []Part{
			TextPart{Text: "我来调用工具"},
			ToolCallPart{ID: "call_1", Name: "weather", Arguments: `{"city":"北京"}`},
			ImagePart{URL: "https://example.com/x.png"},
		},
	}

	// String 只拼接文本
	assert.Equal(t, "我来调用工具", m.String())

	// ToolCalls 提取工具调用
	calls := m.ToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "weather", calls[0].Name)
	assert.Equal(t, "call_1", calls[0].ID)
}

func TestShortcutConstructors(t *testing.T) {
	assert.Equal(t, RoleSystem, System("s").Role)
	assert.Equal(t, "u", User("u").String())
	assert.Equal(t, RoleAssistant, Assistant("a").Role)

	tr := ToolResult("call_9", "ok")
	assert.Equal(t, RoleTool, tr.Role)
	assert.Equal(t, "call_9", tr.ToolCallID)
	assert.Equal(t, "ok", tr.Content[0].(ToolResultPart).Result)
}

func TestChoiceTextAndToolCalls(t *testing.T) {
	c := Choice{Content: []Part{TextPart{Text: "a"}, TextPart{Text: "b"}}}
	assert.Equal(t, "ab", c.Text())
	assert.Nil(t, c.ToolCalls())

	c2 := Choice{Content: []Part{ToolCallPart{ID: "1", Name: "f"}}}
	require.Len(t, c2.ToolCalls(), 1)
}

func TestOptionsApply(t *testing.T) {
	o := Apply(WithTemperature(0.2), WithMaxTokens(100), WithJSONMode(), nil)
	assert.Equal(t, 0.2, o.Temperature)
	assert.Equal(t, 100, o.MaxTokens)
	assert.True(t, o.JSONMode)
	assert.Equal(t, 1, o.N) // 默认值保留
}

func TestFakeModelGenerate(t *testing.T) {
	f := NewFakeModel("第一答", "第二答")

	r1, err := Generate(context.Background(), f, "问题1")
	require.NoError(t, err)
	assert.Equal(t, "第一答", r1)

	r2, err := Generate(context.Background(), f, "问题2")
	require.NoError(t, err)
	assert.Equal(t, "第二答", r2)

	// 耗尽后复读最后一个
	r3, err := Generate(context.Background(), f, "问题3")
	require.NoError(t, err)
	assert.Equal(t, "第二答", r3)

	assert.Equal(t, 3, f.CallCount())
	assert.Len(t, f.LastCall(), 1)
}

func TestFakeModelError(t *testing.T) {
	f := NewFakeModel("x")
	f.Err = ErrRateLimited

	_, err := f.GenerateContent(context.Background(), []Message{User("q")})
	assert.ErrorIs(t, err, ErrRateLimited)

	_, err = f.StreamGenerateContent(context.Background(), []Message{User("q")}, nil)
	assert.ErrorIs(t, err, ErrRateLimited)
}

func TestFakeModelStream(t *testing.T) {
	f := NewFakeModel("你好世界")

	var got string
	resp, err := f.StreamGenerateContent(context.Background(), []Message{User("hi")}, StreamCollector(&got))
	require.NoError(t, err)
	assert.Equal(t, "你好世界", got)
	assert.Equal(t, "你好世界", resp.Choices[0].Text())
	assert.Equal(t, "stop", resp.Choices[0].FinishReason)
}

func TestFakeModelStreamStopEarly(t *testing.T) {
	f := NewFakeModel("abcdef")

	var collected string
	resp, err := f.StreamGenerateContent(context.Background(), []Message{User("q")}, func(c *Chunk) error {
		collected += c.Content
		if len(collected) >= 3 {
			return ErrStopStream // 提前终止
		}
		return nil
	})

	// 返回 ErrStreamClosed，已收内容仍随 Response 返回
	assert.ErrorIs(t, err, ErrStreamClosed)
	assert.Equal(t, "abc", collected)
	assert.Equal(t, "abc", resp.Choices[0].Text())
}

func TestFirstTextEmpty(t *testing.T) {
	_, err := FirstText(&Response{})
	assert.ErrorIs(t, err, ErrEmptyResponse)

	_, err = FirstText(nil)
	assert.ErrorIs(t, err, ErrEmptyResponse)
}

func TestReasonErr(t *testing.T) {
	assert.Nil(t, reasonErr(nil))
	assert.ErrorIs(t, reasonErr(ErrStopStream), ErrStreamClosed)

	other := errors.New("boom")
	assert.Equal(t, other, reasonErr(other))
}

// ============================================================================
// 补充覆盖：选项全量 / Generate 错误路径 / FakeModel 分支 / Part 标记
// ============================================================================

func TestOptionsAllSetters(t *testing.T) {
	o := Apply(
		WithTemperature(0.5), WithMaxTokens(64), WithTopP(0.9),
		WithStop("END", "\n"), WithModel("gpt-x"), WithN(2),
		WithSeed(7), WithTimeout(30*time.Second), WithUser("u123"),
		WithJSONMode(), WithMaxToolIterations(5),
		WithTools(ToolDef{Name: "t"}, ToolDef{Name: "t2"}),
	)
	assert.Equal(t, 0.5, o.Temperature)
	assert.Equal(t, 64, o.MaxTokens)
	assert.Equal(t, 0.9, o.TopP)
	assert.Equal(t, []string{"END", "\n"}, o.Stop)
	assert.Equal(t, "gpt-x", o.Model)
	assert.Equal(t, 2, o.N)
	assert.Equal(t, 7, o.Seed)
	assert.Equal(t, 30*time.Second, o.Timeout)
	assert.Equal(t, "u123", o.User)
	assert.True(t, o.JSONMode)
	assert.Equal(t, 5, o.MaxToolIterations)
	assert.Len(t, o.Tools, 2)
}

func TestGenerateError(t *testing.T) {
	f := &FakeModel{Err: ErrRateLimited}
	_, err := Generate(context.Background(), f, "q")
	assert.ErrorIs(t, err, ErrRateLimited)
}

func TestFakeModelEmptyResponses(t *testing.T) {
	f := &FakeModel{}
	_, err := f.GenerateContent(context.Background(), []Message{User("q")})
	assert.ErrorIs(t, err, ErrEmptyResponse)
}

func TestFakeModelStreamErr(t *testing.T) {
	// StreamErr：吐完文本后返回该错误
	f := NewFakeModel("abc")
	f.StreamErr = ErrProviderUnavailable

	var got string
	_, err := f.StreamGenerateContent(context.Background(), []Message{User("q")}, StreamCollector(&got))
	assert.ErrorIs(t, err, ErrProviderUnavailable)
	assert.Equal(t, "abc", got)
}

func TestFakeModelStreamEmptyResponses(t *testing.T) {
	// 无预设回复的流式：text 为空串 → 逐字符循环零次，直接结束帧
	f := &FakeModel{}
	var frames int
	resp, err := f.StreamGenerateContent(context.Background(), []Message{User("q")}, func(*Chunk) error {
		frames++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, frames) // 仅结束帧
	assert.Empty(t, resp.Choices[0].Text())
}

func TestFakeModelStreamFinishError(t *testing.T) {
	// 结束帧 handler 返回错误
	f := NewFakeModel("x")
	_, err := f.StreamGenerateContent(context.Background(), []Message{User("q")}, func(c *Chunk) error {
		if c.FinishReason != "" {
			return ErrStopStream
		}
		return nil
	})
	assert.ErrorIs(t, err, ErrStreamClosed)
}

func TestFakeModelStreamCustomError(t *testing.T) {
	// handler 返回非 ErrStopStream 错误 → 原样透传（reasonErr 直通）
	f := NewFakeModel("abc")
	boom := errors.New("handler boom")
	_, err := f.StreamGenerateContent(context.Background(), []Message{User("q")}, func(*Chunk) error {
		return boom
	})
	assert.ErrorIs(t, err, boom)
}

func TestFakeModelStreamPopsResponses(t *testing.T) {
	// 流式同样按序消费预设回复
	f := NewFakeModel("第一条", "第二条")

	var got string
	_, err := f.StreamGenerateContent(context.Background(), []Message{User("q")}, StreamCollector(&got))
	require.NoError(t, err)
	assert.Equal(t, "第一条", got)

	got = ""
	_, err = f.StreamGenerateContent(context.Background(), []Message{User("q")}, StreamCollector(&got))
	require.NoError(t, err)
	assert.Equal(t, "第二条", got)
}

func TestFakeModelNilHandler(t *testing.T) {
	// stream 为 nil：与 GenerateContent 等价
	f := NewFakeModel("ok")
	resp, err := f.StreamGenerateContent(context.Background(), []Message{User("q")}, nil)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Choices[0].Text())
}

func TestFakeModelLastCallEmpty(t *testing.T) {
	assert.Nil(t, (&FakeModel{}).LastCall())
}

func TestFakeModelString(t *testing.T) {
	f := NewFakeModel("a", "b")
	f.GenerateContent(context.Background(), []Message{User("q")})
	assert.Contains(t, f.String(), "calls=1")
}

func TestPartMarkers(t *testing.T) {
	// 四种 Part 的 isPart 标记（接口实现编译期 + 运行期确认）
	var parts []Part = []Part{TextPart{}, ImagePart{}, ToolCallPart{}, ToolResultPart{}}
	for _, p := range parts {
		assert.NotNil(t, p)
	}
	// 直接调用未导出标记方法保证覆盖
	TextPart{}.isPart()
	ImagePart{}.isPart()
	ToolCallPart{}.isPart()
	ToolResultPart{}.isPart()
}
