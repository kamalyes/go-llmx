/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 23:23:17
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 23:38:26
 * @FilePath: \go-llmx\adapters\cohere\cohere_test.go
 * @Description: Cohere 适配器单测 —— 请求组装/wire 编解码/流聚合/错误映射
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccohere

import (
	"testing"

	llmx "github.com/kamalyes/go-llmx"
)

func TestBuildRequest_Basics(t *testing.T) {
	msgs := []llmx.Message{llmx.System("be brief"), llmx.User("hi")}
	req := buildRequest(llmx.Apply(llmx.WithTemperature(0.3), llmx.WithTopP(0.9), llmx.WithStop("END")), msgs, false)
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
	if req.Messages[0].Role != roleSystem || req.Messages[0].Content != "be brief" {
		t.Fatalf("system wrong: %+v", req.Messages[0])
	}
	if req.Temperature != 0.3 || req.P != 0.9 || len(req.StopSequences) != 1 {
		t.Fatalf("options lost: %+v", req)
	}
	if req.Stream {
		t.Fatal("non-stream carries stream flag")
	}
}

func TestBuildRequest_ToolMessages(t *testing.T) {
	msgs := []llmx.Message{
		{Role: llmx.RoleAssistant, Content: []llmx.Part{llmx.ToolCallPart{ID: "call1", Name: "get_weather", Arguments: `{"city":"bj"}`}}},
		{Role: llmx.RoleTool, ToolCallID: "call1", Content: []llmx.Part{llmx.ToolResultPart{Result: "sunny"}}},
	}
	req := buildRequest(llmx.DefaultOptions(), msgs, true)
	wa := req.Messages[0]
	if len(wa.ToolCalls) != 1 || wa.ToolCalls[0].Function.Name != "get_weather" || wa.ToolCalls[0].ID != "call1" {
		t.Fatalf("assistant tool call wrong: %+v", wa)
	}
	wt := req.Messages[1]
	results, ok := wt.Content.([]wireToolResult)
	if !ok || len(results) != 1 {
		t.Fatalf("tool content shape wrong: %T %+v", wt.Content, wt.Content)
	}
	if results[0].Type != partTypeToolResult || results[0].ToolCallID != "call1" {
		t.Fatalf("tool_result wrong: %+v", results[0])
	}
	if results[0].Content[0].Text != "sunny" {
		t.Fatalf("result text wrong: %q", results[0].Content[0].Text)
	}
	if !req.Stream {
		t.Fatal("stream flag lost")
	}
}

func TestDecodeResponse_FinishReasons(t *testing.T) {
	cases := map[string]string{
		finishComplete:     "stop",
		finishStopSequence: "stop",
		finishMaxTokens:    "length",
		finishToolCalls:    "tool_calls",
	}
	for in, want := range cases {
		wr := &wireResponse{
			Message:      wireResponseMessage{Content: []wireContentPart{{Type: partTypeText, Text: "ok"}}},
			FinishReason: in,
			Usage:        wireUsage{Tokens: wireTokenCounts{InputTokens: 3, OutputTokens: 5}},
		}
		resp := decodeResponse(wr, "command-r")
		if resp.Choices[0].FinishReason != want {
			t.Fatalf("%s -> %s, want %s", in, resp.Choices[0].FinishReason, want)
		}
		if resp.Usage.TotalTokens != 8 {
			t.Fatalf("usage wrong: %+v", resp.Usage)
		}
	}
}

func TestDecodeResponse_ToolCalls(t *testing.T) {
	wr := &wireResponse{
		Message: wireResponseMessage{
			Content: []wireContentPart{{Type: partTypeText, Text: "checking"}},
			ToolCalls: []wireToolUse{
				{ID: "c1", Type: toolTypeFunction, Function: wireFunctionCall{Name: "f", Arguments: `{"a":1}`}},
			},
		},
		FinishReason: finishToolCalls,
	}
	resp := decodeResponse(wr, "command-r")
	calls := resp.Choices[0].ToolCalls()
	if len(calls) != 1 || calls[0].ID != "c1" || calls[0].Arguments != `{"a":1}` {
		t.Fatalf("calls wrong: %+v", calls)
	}
	if resp.Choices[0].Text() != "checking" {
		t.Fatalf("text = %q", resp.Choices[0].Text())
	}
}

func TestStreamAggregator_TextAndTool(t *testing.T) {
	st := newStreamAggregator()
	var texts, finishes []string
	handler := func(c *llmx.Chunk) error {
		if c.Content != "" {
			texts = append(texts, c.Content)
		}
		if c.FinishReason != "" {
			finishes = append(finishes, c.FinishReason)
		}
		return nil
	}

	frames := []string{
		`{"type":"content-start","index":0}`,
		`{"type":"content-delta","index":0,"delta":{"message":{"content":{"text":"Hel"}}}}`,
		`{"type":"content-delta","index":0,"delta":{"message":{"content":{"text":"lo"}}}}`,
		`{"type":"content-end","index":0}`,
		`{"type":"tool-call-start","index":1,"id":"c1","delta":{"message":{"tool_calls":{"function":{"name":"get_weather"}}}}}`,
		`{"type":"tool-call-delta","index":1,"delta":{"message":{"tool_calls":{"function":{"arguments":"{\"city\":\"bj\"}"}}}}}`,
		`{"type":"tool-call-end","index":1}`,
		`{"type":"message-end","delta":{"finish_reason":"TOOL_CALLS","usage":{"tokens":{"input_tokens":2,"output_tokens":6}}}}`,
	}
	for _, f := range frames {
		if err := st.feed(f, handler); err != nil {
			t.Fatalf("feed: %v", err)
		}
	}
	resp := st.response("command-r")
	if len(texts) != 2 {
		t.Fatalf("text chunks = %d", len(texts))
	}
	if resp.Choices[0].Text() != "Hello" {
		t.Fatalf("text = %q", resp.Choices[0].Text())
	}
	calls := resp.Choices[0].ToolCalls()
	if len(calls) != 1 || calls[0].ID != "c1" || calls[0].Name != "get_weather" || calls[0].Arguments != `{"city":"bj"}` {
		t.Fatalf("call wrong: %+v", calls)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish = %s", resp.Choices[0].FinishReason)
	}
	if resp.Usage.TotalTokens != 8 || len(finishes) != 1 {
		t.Fatalf("usage/finish wrong: %+v %v", resp.Usage, finishes)
	}
}

func TestCohereClassifier_ParseErrorBody(t *testing.T) {
	cls := cohereClassifier{}
	eb := cls.ParseErrorBody(`{"message":"invalid api key"}`)
	if eb.Message != "invalid api key" {
		t.Fatalf("error body wrong: %+v", eb)
	}
	if eb2 := cls.ParseErrorBody("plain text"); eb2.Type != "http_error" {
		t.Fatalf("non-json fallback wrong: %+v", eb2)
	}
}

func TestHeaders_Bearer(t *testing.T) {
	c := New("k")
	if h := c.headers(); h["Authorization"] != "Bearer k" {
		t.Fatalf("auth header wrong: %v", h)
	}
	if c.GetEndpoint() != DefaultBaseURL+ChatPath {
		t.Fatalf("endpoint = %s", c.GetEndpoint())
	}
}
