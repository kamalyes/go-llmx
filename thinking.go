/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-20 21:19:39
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-20 21:29:17
 * @FilePath: \go-llmx\thinking.go
 * @Description: 思考模式控制 —— ThinkingMode 枚举/预算计算/模型能力探测.
 * 请求侧配置（响应侧 Reasoning 字段见 response.go），
 * anthropic 消费 budget_tokens、openai o 系消费 reasoning_effort
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import "strings"

// ThinkingMode 思考模式档位.
// [EN] Thinking mode levels.
type ThinkingMode string

const (
	// ThinkingNone 关闭思考（显式覆盖模型默认行为）.
	// [EN] Disable thinking.
	ThinkingNone ThinkingMode = "none"

	// ThinkingLow 低强度思考（约 20% 输出预算）.
	// [EN] Low effort (~20% of the output budget).
	ThinkingLow ThinkingMode = "low"

	// ThinkingMedium 中强度思考（约 50%）.
	// [EN] Medium effort (~50%).
	ThinkingMedium ThinkingMode = "medium"

	// ThinkingHigh 高强度思考（约 80%）.
	// [EN] High effort (~80%).
	ThinkingHigh ThinkingMode = "high"

	// ThinkingAuto 模型自主决定（provider 默认档位）.
	// [EN] Provider-decided effort.
	ThinkingAuto ThinkingMode = "auto"
)

// Thinking 思考配置（请求级，适配器按协议消费）.
// [EN] Thinking configuration (per-request, consumed per protocol).
type Thinking struct {
	// Mode 档位.
	// [EN] Effort level.
	Mode ThinkingMode

	// BudgetTokens 思考 token 预算（>0 优先于 Mode 档位推导；anthropic 直传）.
	// [EN] Thinking token budget (takes precedence over Mode; passed through by anthropic).
	BudgetTokens int
}

// 预算档位比例（推导用；anthropic 协议要求 budget_tokens 至少 1024）.
// [EN] Budget ratios per level (anthropic requires >= 1024).
const (
	// MinThinkingBudget anthropic 协议思考预算下限.
	// [EN] Protocol floor of the thinking budget.
	MinThinkingBudget = 1024

	// budgetRatioLow/…/budgetRatioHigh 各档位占 MaxTokens 的比例.
	// [EN] Ratio of MaxTokens per level.
	budgetRatioLow    = 0.2
	budgetRatioMed    = 0.5
	budgetRatioHigh   = 0.8
	fallbackMaxTokens = 8192
)

// CalculateThinkingBudget 按 MaxTokens 与档位推导思考预算（未设置 MaxTokens 时按 8192 兜底）.
// [EN] Derive the thinking budget from MaxTokens and the level.
func CalculateThinkingBudget(mode ThinkingMode, maxTokens int) int {
	if maxTokens <= 0 {
		maxTokens = fallbackMaxTokens
	}
	var ratio float64
	switch mode {
	case ThinkingLow:
		ratio = budgetRatioLow
	case ThinkingMedium:
		ratio = budgetRatioMed
	case ThinkingHigh, ThinkingAuto:
		ratio = budgetRatioHigh
	default:
		return 0
	}
	budget := int(float64(maxTokens) * ratio)
	if budget < MinThinkingBudget {
		budget = MinThinkingBudget
	}
	return budget
}

// SupportsReasoning 判定模型名是否具备思考输出能力（已知前缀表；未知模型保守返回 false）.
// [EN] Whether the model name indicates reasoning capability.
func SupportsReasoning(model string) bool {
	if model == "" {
		return false
	}
	lower := strings.ToLower(model)
	prefixes := [...]string{
		// OpenAI o 系
		"o1", "o3", "o4",
		// Anthropic extended thinking
		"claude-3-7", "claude-sonnet-4", "claude-opus-4",
		// DeepSeek
		"deepseek-r1", "deepseek-reasoner",
		// Grok
		"grok-3-mini",
		// Qwen
		"qwq",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	// 通用后缀兜底（如 my-model-thinking）
	return strings.HasSuffix(lower, "-thinking") || strings.HasSuffix(lower, "-reasoner")
}

// WithThinkingMode 设置思考档位（适配器按协议消费；不支持思考的模型静默忽略）.
// [EN] Set the thinking level (silently ignored by non-reasoning models).
func WithThinkingMode(mode ThinkingMode) Option {
	return func(o *Options) {
		o.Thinking = &Thinking{Mode: mode}
	}
}

// WithThinkingBudget 显式设置思考 token 预算（优先于档位推导）.
// [EN] Set an explicit thinking token budget.
func WithThinkingBudget(tokens int) Option {
	return func(o *Options) {
		if tokens < 0 {
			tokens = 0
		}
		o.Thinking = &Thinking{BudgetTokens: tokens}
	}
}
