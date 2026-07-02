/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 21:23:12
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 21:33:58
 * @FilePath: \go-llmx\adapters\mistral\mistral_test.go
 * @Description: Mistral 适配器单测 —— 请求组装/wire 编解码/流聚合/错误映射
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmistral

import (
	"testing"

	llmx "github.com/kamalyes/go-llmx"
)

func TestBuildRequest_Basics(t *testing.T) {
	msgs := []llmx.Message{
		llmx.System("be brief"),
		llmx.User("hi"),
	}
	req := buildRequest(llmx.Apply(llmx.WithTemperature(0.3), llmx.WithMaxTokens(100), llmx.WithStop("END")), msgs, false)
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
	if req.Messages[0].Role != roleSystem || req.Messages[0].Content != "be brief" {
		t.Fatalf("system wrong: %+v", req.Messages[0])
	}
	if req.Temperature != 0.3 || req.MaxTokens != 100 || len(req.Stop) != 1 {
		t.Fatalf("options lost: %+v", req)
	}
	if req.Stream {
		t.Fatal("non-stream request carries stream flag")
	}
}

func TestBuildRequest_ToolCallEncoding(t *testing.T) {
	msgs := []llmx.Message{
		{Role: llmx.RoleAssistant, Content: []llmx.Part{llmx.ToolCallPart{ID: "call1", Name: "get_weather", Arguments: `{"city":"bj"}`}}},
		{Role: llmx.RoleTool, ToolCallID: "call1", Name: "get_weather", Content: []llmx.Part{llmx.ToolResultPart{Result: "sunny"}}},
	}
	req := buildRequest(llmx.DefaultOptions(), msgs, true)
	wm := req.Messages[0]
	if len(wm.ToolCalls) != 1 || wm.ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("tool call wrong: %+v", wm.ToolCalls)
	}
	if wm.ToolCalls[0].Function.Arguments != `{"city":"bj"}` {
		t.Fatalf("args wrong: %s", wm.ToolCalls[0].Function.Arguments)
	}
	if !req.Stream {
		t.Fatal("stream flag lost")
	}
	wt := req.Messages[1]
	if wt.ToolCallID != "call1" || wt.Content != "sunny" {
		t.Fatalf("tool result wrong: %+v", wt)
	}
}

func TestBuildRequest_JSONModeAndTools(t *testing.T) {
	req := buildRequest(llmx.Apply(llmx.WithJSONMode(), llmx.WithTools(llmx.ToolDef{Name: "f", Description: "do", Parameters: map[string]any{"type": "object"}})), []llmx.Message{llmx.User("x")}, false)
	if req.ResponseFormat == nil || req.ResponseFormat.Type != responseFormatJSONObject {
		t.Fatalf("json mode lost: %+v", req.ResponseFormat)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "f" {
		t.Fatalf("tools lost: %+v", req.Tools)
	}
}

func TestDecodeResponse(t *testing.T) {
	wr := &wireResponse{
		Model: "mistral-small-latest",
		Choices: []wireChoice{
			{
				Message: wireChoiceMessage{
					Content: "ok",
					ToolCalls: []wireToolUse{
						{ID: "c1", Type: toolTypeFunction, Function: wireFunctionCall{Name: "f", Arguments: `{"a":1}`}},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: wireUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	}
	resp := decodeResponse(wr, "mistral-small-latest")
	if resp.Choices[0].Text() != "ok" {
		t.Fatalf("text = %q", resp.Choices[0].Text())
	}
	calls := resp.Choices[0].ToolCalls()
	if len(calls) != 1 || calls[0].ID != "c1" || calls[0].Arguments != `{"a":1}` {
		t.Fatalf("calls wrong: %+v", calls)
	}
	if resp.Usage.TotalTokens != 3 {
		t.Fatalf("usage wrong")
	}
}

func TestStreamAggregator_TextThenTool(t *testing.T) {
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
		`{"choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"Hel"}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"lo"}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"get_weather","arguments":"{\"ci"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ty\":\"bj\"}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":2,"completion_tokens":5,"total_tokens":7}}`,
	}
	for _, f := range frames {
		if err := st.feed(f, handler); err != nil {
			t.Fatalf("feed: %v", err)
		}
	}
	resp := st.response("mistral-small-latest")
	if len(texts) != 2 {
		t.Fatalf("text chunks = %d", len(texts))
	}
	if resp.Choices[0].Text() != "Hello" {
		t.Fatalf("text = %q", resp.Choices[0].Text())
	}
	calls := resp.Choices[0].ToolCalls()
	if len(calls) != 1 || calls[0].Name != "get_weather" || calls[0].Arguments != `{"city":"bj"}` {
		t.Fatalf("call wrong: %+v", calls)
	}
	if resp.Choices[0].FinishReason != "tool_calls" || resp.Usage.TotalTokens != 7 {
		t.Fatalf("finish/usage wrong: %+v", resp)
	}
	if len(finishes) != 1 {
		t.Fatalf("finish chunks = %d", len(finishes))
	}
}

func TestMistralClassifier_ParseErrorBody(t *testing.T) {
	cls := mistralClassifier{}
	eb := cls.ParseErrorBody(`{"message":"model not found","type":"invalid_request_error","code":404}`)
	if eb.Type != "invalid_request_error" || eb.Message != "model not found" {
		t.Fatalf("error body wrong: %+v", eb)
	}
	if err := cls.MapErrorType("invalid_request_error"); err != llmx.ErrInvalidRequest {
		t.Fatalf("invalid mapping wrong: %v", err)
	}
	if err := cls.MapErrorType("rate_limit_error"); err != llmx.ErrRateLimited {
		t.Fatalf("rate mapping wrong: %v", err)
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
	if c.GetEndpoint() != DefaultBaseURL+ChatCompletionsPath {
		t.Fatalf("endpoint = %s", c.GetEndpoint())
	}
}
