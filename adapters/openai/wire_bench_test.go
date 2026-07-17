/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-17 10:02:53
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-17 10:02:53
 * @FilePath: \go-llmx\adapters\openai\wire_bench_test.go
 * @Description: 请求构造路径基准 —— encodeMessages/buildRequest/headers
 * 覆盖纯文本快速路径与工具调用混合路径，纯 CPU 零网络
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"encoding/json"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
)

// jsonMarshal/jsonUnmarshal 基准内联的编解码形态（与 DoJSON 路径一致）.
// [EN] Inline codec shape (identical to the DoJSON path).
func jsonMarshal(v any) ([]byte, error)      { return json.Marshal(v) }
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// benchTextMessages 纯文本消息历史（20 轮对话形态）.
// [EN] Plain-text history (20-turn shape).
func benchTextMessages() []llmx.Message {
	msgs := make([]llmx.Message, 20)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = llmx.User("请分析这段系统日志并给出根因推测，包含堆栈与指标上下文")
		} else {
			msgs[i] = llmx.Assistant("根据日志特征，初步判断为连接池耗尽导致的超时级联")
		}
	}
	return msgs
}

// benchToolMessages 工具调用混合历史（agent 多轮形态）.
// [EN] Tool-call mixed history (agent shape).
func benchToolMessages() []llmx.Message {
	msgs := make([]llmx.Message, 24)
	i := 0
	for round := 0; round < 6; round++ {
		msgs[i] = llmx.User("查询北京天气并计算体感温度")
		i++
		msgs[i] = llmx.Assistant("我来查询天气")
		msgs[i].Content = []llmx.Part{
			llmx.ToolCallPart{ID: "c1", Name: "weather", Arguments: `{"city":"北京"}`},
			llmx.ToolCallPart{ID: "c2", Name: "calculator", Arguments: `{"expression":"25*1.1"}`},
		}
		i++
		msgs[i] = llmx.ToolResult("c1", `{"temp":25,"humidity":60}`)
		i++
		msgs[i] = llmx.ToolResult("c2", "27.5")
		i++
	}
	return msgs[:i]
}

// benchOpts 典型请求选项.
// [EN] Typical request options.
func benchOpts() []llmx.Option {
	return []llmx.Option{llmx.WithTemperature(0.7)}
}

// BenchmarkEncodeMessages_Text 纯文本快速路径.
// [EN] Plain-text fast path.
func BenchmarkEncodeMessages_Text(b *testing.B) {
	msgs := benchTextMessages()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if out := encodeMessages(msgs); len(out) != 20 {
			b.Fatal("bad len")
		}
	}
}

// BenchmarkEncodeMessages_Tool 工具调用混合路径.
// [EN] Tool-call mixed path.
func BenchmarkEncodeMessages_Tool(b *testing.B) {
	msgs := benchToolMessages()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if out := encodeMessages(msgs); len(out) != 24 {
			b.Fatal("bad len")
		}
	}
}

// BenchmarkBuildRequest 完整请求组装（编码+选项消费）.
// [EN] Full request assembly.
func BenchmarkBuildRequest(b *testing.B) {
	msgs := benchToolMessages()
	opts := benchOpts()
	c := New("sk-bench")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if req := c.buildRequest(llmx.Apply(opts...), msgs, false); req == nil {
			b.Fatal("nil req")
		}
	}
}

// BenchmarkHeaders 认证头构造（每请求一次）.
// [EN] Auth header construction (per request).
func BenchmarkHeaders(b *testing.B) {
	c := New("sk-bench")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if h := c.headers(); h == nil {
			b.Fatal("nil headers")
		}
	}
}

// BenchmarkMarshalRequest 序列化 wire 请求（DoJSON 内 json.Marshal 形态）.
// [EN] Wire request serialization (json.Marshal shape).
func BenchmarkMarshalRequest(b *testing.B) {
	msgs := benchToolMessages()
	c := New("sk-bench")
	req := c.buildRequest(llmx.Apply(benchOpts()...), msgs, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := jsonMarshal(req); err != nil {
			b.Fatal(err)
		}
	}
}

// benchDecodeChoiceInput 响应解码基准的固定输入（1 个 choice）.
// [EN] Fixed input for response decoding.
var benchDecodeChoiceInput = []byte(`{"choices":[{"message":{"role":"assistant","content":"根据日志特征判断为连接池耗尽"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1024,"completion_tokens":256,"total_tokens":1280}}`)

// BenchmarkDecodeResponse 响应解码（json.Unmarshal wireResponse + decodeChoice）.
// [EN] Response decoding.
func BenchmarkDecodeResponse(b *testing.B) {
	c := New("k")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wr wireResponse
		if err := jsonUnmarshal(benchDecodeChoiceInput, &wr); err != nil {
			b.Fatal(err)
		}
		resp := &llmx.Response{Model: c.Model}
		for _, ch := range wr.Choices {
			resp.Choices = append(resp.Choices, decodeChoice(ch))
		}
	}
}
