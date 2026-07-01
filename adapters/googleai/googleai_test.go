/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-01 22:21:17
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-01 22:36:08
 * @FilePath: \go-llmx\adapters\googleai\googleai_test.go
 * @Description: Gemini 适配器单测 —— 请求组装/wire 编解码/流聚合/错误映射
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgoogleai

import (
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/transport"
)

func TestBuildRequest_SystemMerged(t *testing.T) {
	msgs := []llmx.Message{
		llmx.System("be brief"),
		llmx.User("hi"),
	}
	req := buildRequest(llmx.Apply(llmx.WithTemperature(0.2)), msgs)
	if req.SystemInstruction == nil || len(req.SystemInstruction.Parts) != 1 {
		t.Fatalf("system not merged: %+v", req.SystemInstruction)
	}
	if req.SystemInstruction.Parts[0].Text != "be brief" {
		t.Fatalf("system text lost: %q", req.SystemInstruction.Parts[0].Text)
	}
	if len(req.Contents) != 1 || req.Contents[0].Role != roleUser {
		t.Fatalf("user content wrong: %+v", req.Contents)
	}
	if req.GenerationConfig == nil || req.GenerationConfig.Temperature != 0.2 {
		t.Fatalf("generation config lost")
	}
}

func TestBuildRequest_ToolResultAsUserEntry(t *testing.T) {
	msgs := []llmx.Message{
		llmx.User("weather?"),
		{Role: llmx.RoleAssistant, Content: []llmx.Part{llmx.ToolCallPart{ID: "get_weather", Name: "get_weather", Arguments: `{"city":"bj"}`}}},
		{Role: llmx.RoleTool, ToolCallID: "get_weather", Name: "get_weather", Content: []llmx.Part{llmx.ToolResultPart{Result: "sunny"}}},
	}
	req := buildRequest(llmx.DefaultOptions(), msgs)
	if len(req.Contents) != 3 {
		t.Fatalf("contents len = %d", len(req.Contents))
	}
	last := req.Contents[2]
	if last.Role != roleUser {
		t.Fatalf("tool result role = %s, want user", last.Role)
	}
	fr := last.Parts[0].FunctionResponse
	if fr == nil || fr.Name != "get_weather" || fr.Response["result"] != "sunny" {
		t.Fatalf("functionResponse wrong: %+v", fr)
	}
	mid := req.Contents[1]
	if mid.Role != roleModel || mid.Parts[0].FunctionCall == nil {
		t.Fatalf("assistant tool call wrong: %+v", mid)
	}
	if string(mid.Parts[0].FunctionCall.Args) != `{"city":"bj"}` {
		t.Fatalf("args lost: %s", mid.Parts[0].FunctionCall.Args)
	}
}

func TestBuildRequest_JSONModeAndThinking(t *testing.T) {
	req := buildRequest(llmx.Apply(llmx.WithJSONMode(), llmx.WithThinkingBudget(512)), []llmx.Message{llmx.User("x")})
	if req.GenerationConfig.ResponseMimeType != mimeJSON {
		t.Fatalf("json mode not mapped: %s", req.GenerationConfig.ResponseMimeType)
	}
	if req.GenerationConfig.ThinkingConfig == nil || req.GenerationConfig.ThinkingConfig.ThinkingBudget != 512 {
		t.Fatalf("thinking not mapped: %+v", req.GenerationConfig.ThinkingConfig)
	}
}

func TestDecodeResponse_FinishReasons(t *testing.T) {
	wr := &wireResponse{
		Candidates: []wireCandidate{
			{Content: &wireContent{Parts: []wirePart{{Text: "ok"}}}, FinishReason: finishStop},
			{Content: &wireContent{Parts: []wirePart{{FunctionCall: &wireFunctionCall{Name: "f"}}}}, FinishReason: finishStop},
			{Content: &wireContent{Parts: []wirePart{{Text: "cut"}}}, FinishReason: finishMaxTokens},
			{Content: &wireContent{Parts: []wirePart{{Text: "no"}}}, FinishReason: finishSafety},
		},
		UsageMetadata: &wireUsageMetadata{PromptTokenCount: 3, CandidatesTokenCount: 5, TotalTokenCount: 8},
		ModelVersion:  "gemini-2.0-flash-001",
	}
	resp := decodeResponse(wr, "gemini-2.0-flash")
	if len(resp.Choices) != 4 {
		t.Fatalf("choices = %d", len(resp.Choices))
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("reason0 = %s", resp.Choices[0].FinishReason)
	}
	if resp.Choices[1].FinishReason != "tool_calls" {
		t.Fatalf("tool frame should map to tool_calls, got %s", resp.Choices[1].FinishReason)
	}
	if resp.Choices[1].ToolCalls()[0].Name != "f" {
		t.Fatalf("tool call lost")
	}
	if resp.Choices[2].FinishReason != "length" {
		t.Fatalf("reason2 = %s", resp.Choices[2].FinishReason)
	}
	if resp.Choices[3].FinishReason != "content_filter" {
		t.Fatalf("reason3 = %s", resp.Choices[3].FinishReason)
	}
	if resp.Usage.TotalTokens != 8 || resp.Model != "gemini-2.0-flash-001" {
		t.Fatalf("usage/model wrong: %+v %s", resp.Usage, resp.Model)
	}
}

func TestStreamAggregator_TextAndTool(t *testing.T) {
	st := newStreamAggregator()
	var texts []string
	handler := func(c *llmx.Chunk) error {
		if c.Content != "" {
			texts = append(texts, c.Content)
		}
		return nil
	}

	frame1 := `{"candidates":[{"content":{"parts":[{"text":"Hel"}]}}]}`
	frame2 := `{"candidates":[{"content":{"parts":[{"text":"lo"}]}}]}`
	frame3 := `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_weather","args":{"city":"bj"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":6,"totalTokenCount":8}}`

	for _, f := range []string{frame1, frame2, frame3} {
		if err := st.feed(transport.SSEEvent{Data: f}, handler); err != nil {
			t.Fatalf("feed: %v", err)
		}
	}
	resp := st.response("gemini-2.0-flash")
	if got := len(texts); got != 2 {
		t.Fatalf("text chunks = %d", got)
	}
	if resp.Choices[0].Text() != "Hello" {
		t.Fatalf("text = %q", resp.Choices[0].Text())
	}
	if len(resp.Choices[0].ToolCalls()) != 1 || resp.Usage.TotalTokens != 8 {
		t.Fatalf("tool/usage wrong: %+v", resp)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish = %s", resp.Choices[0].FinishReason)
	}
}

func TestGoogleaiClassifier_ParseErrorBody(t *testing.T) {
	cls := googleaiClassifier{}
	body := `{"error":{"code":429,"message":"quota exceeded","status":"RESOURCE_EXHAUSTED"}}`
	eb := cls.ParseErrorBody(body)
	if eb.Type != "RESOURCE_EXHAUSTED" || eb.Message != "quota exceeded" {
		t.Fatalf("error body wrong: %+v", eb)
	}
	if err := cls.MapErrorType("RESOURCE_EXHAUSTED"); err != llmx.ErrRateLimited {
		t.Fatalf("rate limit mapping wrong: %v", err)
	}
	if err := cls.MapErrorType("UNAUTHENTICATED"); err != llmx.ErrUnauthorized {
		t.Fatalf("auth mapping wrong: %v", err)
	}
	if eb2 := cls.ParseErrorBody("plain text"); eb2.Type != "http_error" {
		t.Fatalf("non-json fallback wrong: %+v", eb2)
	}
}

func TestEndpoint_ModelInPath(t *testing.T) {
	c := New("k")
	if got := c.endpoint(DefaultModel, methodGenerateContent); got != DefaultBaseURL+"/models/"+DefaultModel+":generateContent" {
		t.Fatalf("endpoint = %s", got)
	}
	if h := c.headers(); h[keyAPIKey] != "k" {
		t.Fatalf("auth header wrong: %v", h)
	}
}

func TestValidateMessages(t *testing.T) {
	if err := validateMessages(nil); err == nil {
		t.Fatal("empty list should fail")
	}
	if err := validateMessages([]llmx.Message{{Role: llmx.RoleUser}}); err == nil {
		t.Fatal("empty content should fail")
	}
	if err := validateMessages([]llmx.Message{llmx.User("hi")}); err != nil {
		t.Fatalf("valid messages rejected: %v", err)
	}
}
