/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-02 09:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-02 09:30:00
 * @FilePath: \go-llmx\adapters\openai\examples\deepseek\main.go
 * @Description: DeepSeek 真实调用示例 —— OpenAI 兼容协议直连 DeepSeek 官方端点.
 * 演示非流式/流式/工具调用三段链路；密钥从环境变量读取，代码零硬编码
 *
 * 运行前：
 *   export DEEPSEEK_API_KEY=sk-xxx        # 必填
 *   export DEEPSEEK_MODEL=deepseek-chat   # 可选，缺省 deepseek-chat（reasoner 同样可跑）
 *   export DEEPSEEK_BASE_URL=https://api.deepseek.com  # 可选，代理/网关时覆盖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	llmx "github.com/kamalyes/go-llmx"
	lcopenai "github.com/kamalyes/go-llmx/adapters/openai"
	"github.com/kamalyes/go-llmx/tool"
)

// 环境变量名与默认值.
// [EN] Environment variables and defaults.
const (
	envAPIKey  = "DEEPSEEK_API_KEY"
	envModel   = "DEEPSEEK_MODEL"
	envBaseURL = "DEEPSEEK_BASE_URL"

	defaultModel   = "deepseek-chat"
	defaultBaseURL = "https://api.deepseek.com"
)

func main() {
	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "缺少环境变量 %s（DeepSeek 开放平台获取：https://platform.deepseek.com）\n", envAPIKey)
		os.Exit(1)
	}
	model := getenv(envModel, defaultModel)
	baseURL := getenv(envBaseURL, defaultBaseURL)

	// DeepSeek 是 OpenAI 兼容协议：换 baseURL + 模型名即接入，其余零改动
	c := lcopenai.New(apiKey, lcopenai.WithBaseURL(baseURL), lcopenai.WithModel(model))

	section("客户端装配")
	fmt.Printf("端点: %s\n模型: %s\n", c.GetEndpoint(), c.GetModel())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	demoNonStream(ctx, c)
	demoStream(ctx, c)
	demoToolLoop(ctx, c)
}

// demoNonStream 非流式：一次请求拿全量回答与用量.
// [EN] Non-streaming: one request for the full answer and usage.
func demoNonStream(ctx context.Context, c *lcopenai.Client) {
	section("① 非流式调用")
	resp, err := c.GenerateContent(ctx, []llmx.Message{
		llmx.System("你是一名简洁的 Go 技术专家，回答控制在两句话以内"),
		llmx.User("go-llmx 的哨兵错误设计好在哪里？"),
	})
	fatalIf(err, "GenerateContent")

	text, _ := llmx.FirstText(resp)
	fmt.Printf("回答: %s\n", text)
	fmt.Printf("结束原因: %s | 实际模型: %s\n", resp.Choices[0].FinishReason, resp.Model)
	fmt.Printf("用量: prompt=%d completion=%d total=%d\n",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

// demoStream 流式：增量逐段打印（reasoner 模型的思考轨迹以灰显前缀输出）.
// [EN] Streaming: print deltas as they arrive (reasoning dimmed for reasoner).
func demoStream(ctx context.Context, c *lcopenai.Client) {
	section("② 流式调用")
	var full strings.Builder
	resp, err := c.StreamGenerateContent(ctx,
		[]llmx.Message{llmx.User("用三句话介绍一下你自己")},
		func(chunk *llmx.Chunk) error {
			if chunk.Reasoning != "" {
				// 思考轨迹增量（deepseek-reasoner；deepseek-chat 恒为空）
				fmt.Printf("\r%s", ansiDim("思考中: "+truncate(chunk.Reasoning, 30)))
				return nil
			}
			fmt.Print(chunk.Content)
			full.WriteString(chunk.Content)
			return nil
		})
	fmt.Println()
	fatalIf(err, "StreamGenerateContent")

	fmt.Printf("聚合文本: %s\n", truncate(full.String(), 60))
	fmt.Printf("结束原因: %s | 流末用量: total=%d\n",
		resp.Choices[0].FinishReason, resp.Usage.TotalTokens)
}

// demoToolLoop 工具调用循环：模型请求 → 执行 → 回传 → 最终回答.
// [EN] Tool loop: model requests, we execute, answer returned.
func demoToolLoop(ctx context.Context, c *lcopenai.Client) {
	section("③ 工具调用循环")

	// Go 结构体 → JSON Schema（零依赖反射生成）
	type weatherArgs struct {
		City string `json:"city"`
		Unit string `json:"unit,omitempty"`
	}
	tools := []tool.Tool{{
		Name:        "weather",
		Description: "查询指定城市的实时天气",
		Parameters:  tool.SchemaOf(weatherArgs{}),
		Func: func(ctx context.Context, arguments string) (string, error) {
			// 演示用静态数据；真实场景替换为天气 API 调用
			fmt.Printf("%s 工具被调用 → 参数: %s\n", ansiGreen("⚙"), arguments)
			return `{"city":"北京","temp":25,"condition":"晴","humidity":40}`, nil
		},
	}}

	resp, err := tool.RunToolLoop(ctx, c,
		[]llmx.Message{llmx.User("北京今天天气怎么样？适合户外跑步吗")},
		tools)
	fatalIf(err, "RunToolLoop")

	for _, call := range resp.Choices[0].ToolCalls() {
		fmt.Printf("模型发起调用: %s(%s)\n", call.Name, call.Arguments)
	}
	text, _ := llmx.FirstText(resp)
	fmt.Printf("最终回答: %s\n", text)
}

// fatalIf 错误出口：哨兵分类呈现后终止.
// [EN] Error exit: classify by sentinel then terminate.
func fatalIf(err error, where string) {
	if err == nil {
		return
	}
	var hint string
	switch {
	case errors.Is(err, llmx.ErrUnauthorized):
		hint = "密钥无效或已过期，请检查 DEEPSEEK_API_KEY"
	case errors.Is(err, llmx.ErrRateLimited):
		hint = "触发限流，稍后重试"
	case errors.Is(err, llmx.ErrProviderUnavailable):
		hint = "网络不可达或端点错误，请检查 DEEPSEEK_BASE_URL"
	case errors.Is(err, llmx.ErrInvalidRequest):
		hint = "请求参数非法（如模型名不存在），请检查 DEEPSEEK_MODEL"
	case errors.Is(err, llmx.ErrAPIServerError):
		hint = "DeepSeek 服务端错误，稍后重试"
	}
	fmt.Fprintf(os.Stderr, "%s 失败: %v\n%s\n", where, err, ansiRed("处置建议: "+hint))
	os.Exit(1)
}

// getenv 读环境变量，空值回落默认.
// [EN] Read env with fallback.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// section 打印分节标题.
// [EN] Print a section header.
func section(title string) {
	fmt.Printf("\n%s %s %s\n", ansiCyan("━"[:0]), ansiCyan(strings.Repeat("━", 8)), ansiBold(title))
}

// truncate 截断长文本用于单行展示.
// [EN] Truncate long text for single-line display.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// ANSI 颜色（Windows Terminal / 现代终端均支持）.
// [EN] ANSI colors (Windows Terminal friendly).
func ansiDim(s string) string   { return "\x1b[2m" + s + "\x1b[0m" }
func ansiGreen(s string) string { return "\x1b[32m" + s + "\x1b[0m" }
func ansiRed(s string) string   { return "\x1b[31m" + s + "\x1b[0m" }
func ansiCyan(s string) string  { return "\x1b[36m" + s + "\x1b[0m" }
func ansiBold(s string) string  { return "\x1b[1m" + s + "\x1b[0m" }
