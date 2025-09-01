/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-22 09:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-22 09:30:00
 * @FilePath: \go-llmx\adapters\openai\examples\streamtools\main.go
 * @Description: 流式回调中处理工具调用参数的完整演示（mock SSE，无需密钥）.
 * 内置模拟服务器按 OpenAI 协议下发：思考轨迹 → 双工具并行组装（参数分段 + 乱序 index）
 * → 结束帧；handler 侧演示按 Index 归位、实时进度与最终聚合取用
 *
 * 运行：go run ./examples/streamtools
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
	lcopenai "github.com/kamalyes/go-llmx/adapters/openai"
)

// toolProgress 单个工具调用的组装进度（按 Index 归位追踪）.
// [EN] Assembly progress of one tool call (tracked by Index).
type toolProgress struct {
	index int
	id    string
	name  string
	args  strings.Builder
}

// streamToolWatcher 流式处理器：回调期间按 Index 维护进行中的工具调用.
// [EN] Streaming handler: track in-flight tool calls by Index.
type streamToolWatcher struct {
	live map[int]*toolProgress
}

// newStreamToolWatcher 构造监视器.
// [EN] Build the watcher.
func newStreamToolWatcher() *streamToolWatcher {
	return &streamToolWatcher{live: map[int]*toolProgress{}}
}

// handle 处理一帧增量（工具帧归位追踪，文本帧透传打印）.
// [EN] Handle one delta frame (tool frames tracked, text frames printed).
func (w *streamToolWatcher) handle(chunk *llmx.Chunk) error {
	// 思考轨迹增量（reasoner 类模型）
	if chunk.Reasoning != "" {
		fmt.Printf("\r[思考] %.50s", chunk.Reasoning)
		return nil
	}

	// 文本增量
	if chunk.Content != "" {
		fmt.Printf("\r%s\r[回答] %s", strings.Repeat(" ", 60), chunk.Content)
		return nil
	}

	// 工具调用增量：首帧带 ID/Name，后续帧仅带 Arguments 片段
	if d := chunk.ToolCallDelta; d != nil {
		p, ok := w.live[d.Index]
		if !ok {
			// 首帧：登记新调用（乱序防御——按 Index 归位而非到达顺序）
			p = &toolProgress{index: d.Index, id: d.ID, name: d.Name}
			w.live[d.Index] = p
			fmt.Printf("\n  [工具开始] index=%d name=%s id=%s\n", d.Index, d.Name, d.ID)
		}
		if d.Arguments != "" {
			p.args.WriteString(d.Arguments) // 逐段追加（本帧片段，非全量）
			fmt.Printf("  [参数追加] index=%d 片段=%-16s 已拼=%s\n",
				d.Index, d.Arguments, p.args.String())
		}
		return nil
	}

	// 结束帧：携带 FinishReason + Usage
	if chunk.FinishReason != "" {
		fmt.Printf("\n  [流结束] finish=%s usage{prompt=%d completion=%d}\n",
			chunk.FinishReason, chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens)
	}
	return nil
}

// summarize 按序输出各调用的最终聚合状态（回调侧自行拼接）.
// [EN] Print the final assembled state of each call (joined on the callback side).
func (w *streamToolWatcher) summarize() {
	idx := make([]int, 0, len(w.live))
	for i := range w.live {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	fmt.Printf("\n===== 回调侧自行拼接的结果（应与聚合器一致）=====\n")
	for _, i := range idx {
		p := w.live[i]
		var args map[string]any
		if err := json.Unmarshal([]byte(p.args.String()), &args); err != nil {
			fmt.Printf("  [%d] %s 参数非法 JSON: %s\n", p.index, p.name, p.args.String())
			continue
		}
		fmt.Printf("  [%d] %s(%s) 完整参数: %v\n", p.index, p.name, p.id, args)
	}
}

func main() {
	m := newMockStreamServer()
	defer m.Close()

	// 注入 go-logger：演示 [LLMX] ContextKV 打点（stream 模式 + transport http 维度）
	c := lcopenai.New("demo-key",
		lcopenai.WithBaseURL(m.URL),
		lcopenai.WithModel("deepseek-chat"),
	)

	fmt.Println("===== 流式工具调用参数处理演示（mock SSE，无需密钥）=====")
	w := newStreamToolWatcher()
	resp, err := c.StreamGenerateContent(context.Background(),
		[]llmx.Message{llmx.User("北京和上海今天天气怎么样")},
		w.handle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "流式调用失败: %v\n", err)
		os.Exit(1)
	}

	// ① 回调侧自行拼接的最终状态（范式：自建 map + Index 归位 + 逐段追加）
	w.summarize()

	// ② 聚合器侧：流结束后 Response 已含拼装完整的调用（无需自己拼）
	fmt.Printf("\n===== 聚合器侧（resp.Choices[0].ToolCalls()）=====\n")
	for _, call := range resp.Choices[0].ToolCalls() {
		var args map[string]any
		_ = json.Unmarshal([]byte(call.Arguments), &args)
		fmt.Printf("  %s(%s) 参数: %v\n", call.Name, call.ID, args)
	}
	fmt.Printf("聚合文本: %q | 思考: %q | 结束: %s\n",
		resp.Choices[0].Text(), resp.Choices[0].Reasoning, resp.Choices[0].FinishReason)
}

// newMockStreamServer 模拟 OpenAI 兼容 SSE：思考 → 双工具（乱序 index + 参数分段交织）→ 结束.
// [EN] Mock OpenAI SSE: thinking, two tools (out-of-order index, interleaved args), end frame.
func newMockStreamServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		frame := func(s string) { fmt.Fprintf(w, "data: %s\n\n", s) }

		// 思考轨迹帧（deepseek-reasoner 风格）
		frame(`{"choices":[{"delta":{"reasoning_content":"用户要查两个城市，我需要并行调用两次天气工具"}}]}`)
		// 文本帧
		frame(`{"choices":[{"delta":{"content":"我来查一下两地天气。"}}]}`)

		// 工具 A（index=0）：首帧带 id/name + 参数首段（分段未闭合，末段才收口）
		frame(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"weather","arguments":"{\"ci"}}]}}]}`)
		frame(`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ty\":\"北京\",\"unit\":\"c\""}}]}}]}`)

		// 工具 B（index=1）：乱序穿插到达（多工具并行组装的常规形态）
		frame(`{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","function":{"name":"weather","arguments":"{\"city\":\"上海\",\"unit"}}]}}]}`)
		frame(`{"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"\":\"c\"}"}}]}}]}`)

		// 参数续段可能再回到工具 A（模型自由交织）
		frame(`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":",\"lang\":\"zh\"}"}}]}}]}`)

		// 结束帧 + 用量
		frame(`{"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":42,"completion_tokens":18,"total_tokens":60}}`)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}
