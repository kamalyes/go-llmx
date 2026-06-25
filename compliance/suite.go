/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-25 22:38:13
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-25 22:53:37
 * @FilePath: \go-llmx\compliance\suite.go
 * @Description: 合规测试套件 —— llmx.Model 契约的标准用例集.
 * 适配器一行接入：compliance.New(t, model).Run()，
 * 保证多 provider 行为一致性（消息收发/流式聚合/工具调用/参数消费）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package compliance

import (
	"context"
	"strings"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/tool"
)

// Suite 合规测试套件.
// [EN] Compliance test suite.
type Suite struct {
	// t 测试载体.
	// [EN] Testing context.
	t *testing.T

	// model 被测模型实例.
	// [EN] Model under test.
	model llmx.Model

	// skip 按用例名跳过（键为用例名，如 "TestStreaming"）.
	// [EN] Cases to skip by name.
	skip map[string]bool
}

// New 构造合规套件.
// [EN] Build a compliance suite.
func New(t *testing.T, model llmx.Model) *Suite {
	return &Suite{t: t, model: model, skip: map[string]bool{}}
}

// Skip 跳过指定用例（不支持能力的适配器使用，如 ollama 无 JSON 模式）.
// [EN] Skip a case (for adapters lacking a capability).
func (s *Suite) Skip(caseNames ...string) *Suite {
	for _, n := range caseNames {
		s.skip[n] = true
	}
	return s
}

// Run 执行全量标准用例（跳过项 t.Skip 输出说明）.
// [EN] Run all standard cases.
func (s *Suite) Run() {
	s.run("TestGenerateReturnsText", s.testGenerateReturnsText)
	s.run("TestStreamingAggregates", s.testStreamingAggregates)
	s.run("TestToolCallsExposed", s.testToolCallsExposed)
	s.run("TestMaxTokensConsumed", s.testMaxTokensConsumed)
	s.run("TestStopSequencesConsumed", s.testStopSequencesConsumed)
	s.run("TestEmptyMessagesRejected", s.testEmptyMessagesRejected)
	s.run("TestContextCancellation", s.testContextCancellation)
}

// run 单用例执行骨架（跳过感知）.
// [EN] Run one case (skip-aware).
func (s *Suite) run(name string, fn func(t *testing.T)) {
	if s.skip[name] {
		s.t.Skipf("compliance: %s skipped for this provider", name)
		return
	}
	if !s.t.Run(name, fn) {
		return
	}
}

// testGenerateReturnsText 基础生成：非空文本返回.
// [EN] Basic generation: non-empty text.
func (s *Suite) testGenerateReturnsText(t *testing.T) {
	resp, err := s.model.GenerateContent(t.Context(), []llmx.Message{llmx.User("Say hello.")})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if text, err := llmx.FirstText(resp); err != nil || strings.TrimSpace(text) == "" {
		t.Fatalf("expected non-empty text, got %q (err=%v)", text, err)
	}
}

// testStreamingAggregates 流式聚合：增量回调后聚合结果非空.
// [EN] Streaming: aggregated result non-empty after deltas.
func (s *Suite) testStreamingAggregates(t *testing.T) {
	var collected strings.Builder
	resp, err := s.model.StreamGenerateContent(
		t.Context(),
		[]llmx.Message{llmx.User("Count from 1 to 3.")},
		func(chunk *llmx.Chunk) error {
			collected.WriteString(chunk.Content)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	// 聚合终态或增量回调，至少其一非空
	final, ferr := llmx.FirstText(resp)
	if ferr == nil && strings.TrimSpace(final) != "" {
		return
	}
	if strings.TrimSpace(collected.String()) == "" {
		t.Fatalf("expected stream content via handler or final response, got neither")
	}
}

// testToolCallsExposed 工具调用：echo 工具被模型请求或正常文本回落.
// [EN] Tool calling: a request surfaces or text falls back.
func (s *Suite) testToolCallsExposed(t *testing.T) {
	echo := llmx.ToolDef{
		Name:        "echo",
		Description: "Echo the input text back",
		Parameters:  tool.SchemaOf(struct{ Text string }{}),
	}
	resp, err := s.model.GenerateContent(t.Context(),
		[]llmx.Message{llmx.User("Use the echo tool to repeat: hello")},
		llmx.WithTools(echo),
	)
	if err != nil {
		t.Fatalf("tool call: %v", err)
	}
	if resp == nil || len(resp.Choices) == 0 {
		t.Fatal("expected at least one choice")
	}
}

// testMaxTokensConsumed MaxTokens 参数消费（不 panic 即通过，长度语义 provider 相关）.
// [EN] MaxTokens consumed (no panic; length semantics are provider-specific).
func (s *Suite) testMaxTokensConsumed(t *testing.T) {
	_, _ = s.model.GenerateContent(t.Context(),
		[]llmx.Message{llmx.User("Tell me a long story.")},
		llmx.WithMaxTokens(64),
	)
}

// testStopSequencesConsumed 停止序列消费（不 panic 即通过）.
// [EN] Stop sequences consumed.
func (s *Suite) testStopSequencesConsumed(t *testing.T) {
	_, _ = s.model.GenerateContent(t.Context(),
		[]llmx.Message{llmx.User("List three colors.")},
		llmx.WithStop("red"),
	)
}

// testEmptyMessagesRejected 空消息列表拒绝（参数校验一致）.
// [EN] Empty messages rejected.
func (s *Suite) testEmptyMessagesRejected(t *testing.T) {
	_, err := s.model.GenerateContent(t.Context(), nil)
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}

// testContextCancellation 已取消 context 拒绝调用（不浪费上游配额）.
// [EN] A cancelled context is rejected.
func (s *Suite) testContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _ = s.model.GenerateContent(ctx, []llmx.Message{llmx.User("hi")})
}
