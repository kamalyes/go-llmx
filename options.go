/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-15 21:35:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-20 21:29:17
 * @FilePath: \go-llmx\options.go
 * @Description: 函数式调用选项 —— Option 作用于单次 GenerateContent 调用.
 * 模型/端点等长生命周期配置由适配器构造函数承载，此处只管请求级参数
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package llmx

import "time"

// Option 单次生成的函数式选项.
// [EN] Functional option for a single generation call.
type Option func(*Options)

// Options 单次生成的全部可调参数（适配器按能力消费，不支持的项静默忽略）.
// [EN] All tunable parameters for one generation call.
type Options struct {
	// Temperature 采样温度（0 确定性 ~ 2 发散）.
	Temperature float64

	// MaxTokens 输出 token 上限（0 = provider 默认）.
	MaxTokens int

	// TopP 核采样概率阈值（0 = provider 默认）.
	TopP float64

	// Stop 命中即停止的序列（最多 4 个，对齐 OpenAI 限制）.
	Stop []string

	// Model 覆盖客户端默认模型（一次性行为，如临时切换到更强模型）.
	Model string

	// N 一次生成的候选数（默认 1）.
	N int

	// Seed 随机种子（支持确定性输出的 provider 消费）.
	Seed int

	// Timeout 单次请求超时（覆盖客户端默认；0 = 客户端默认）.
	Timeout time.Duration

	// JSONMode 强制 JSON 输出（provider 支持时设置 response_format）.
	JSONMode bool

	// User 终端用户标识（provider 风控/计费归属用）.
	User string

	// Tools 本次调用暴露给模型的工具定义（无则不携带 tools 参数）.
	Tools []ToolDef

	// MaxToolIterations 工具调用循环上限（RunToolLoop 消费；适配器不消费此字段；<=0 走默认）.
	// [EN] Tool loop limit (consumed by RunToolLoop; adapters ignore it).
	MaxToolIterations int

	// Thinking 思考配置（nil = 不启用；支持思考的适配器消费，见 thinking.go）.
	// [EN] Thinking configuration (nil = disabled; consumed by reasoning-capable adapters).
	Thinking *Thinking
}

// ToolDef 工具定义（暴露给模型的函数签名）.
// [EN] Tool definition exposed to the model.
type ToolDef struct {
	// Name 工具名（模型调用时引用）.
	Name string

	// Description 工具功能描述（模型据此决定是否调用）.
	Description string

	// Parameters 参数 JSON Schema（结构体或 map，由适配器序列化）.
	Parameters any

	// Strict 严格模式（OpenAI structured outputs 保证参数合法）.
	Strict bool
}

// WithTools 携带工具定义.
// [EN] Attach tool definitions to the call.
func WithTools(defs ...ToolDef) Option {
	return func(o *Options) { o.Tools = append(o.Tools, defs...) }
}

// DefaultOptions 默认参数：温度 0.7、单候选、其余零值走 provider 默认.
// [EN] Default parameters: temperature 0.7, single choice.
func DefaultOptions() *Options {
	return &Options{
		Temperature: 0.7,
		N:           1,
	}
}

// WithTemperature 设置采样温度.
// [EN] Set sampling temperature.
func WithTemperature(t float64) Option {
	return func(o *Options) { o.Temperature = t }
}

// WithMaxTokens 设置输出 token 上限.
// [EN] Set max output tokens.
func WithMaxTokens(n int) Option {
	return func(o *Options) { o.MaxTokens = n }
}

// WithTopP 设置核采样阈值.
// [EN] Set nucleus sampling threshold.
func WithTopP(p float64) Option {
	return func(o *Options) { o.TopP = p }
}

// WithStop 设置停止序列.
// [EN] Set stop sequences.
func WithStop(seqs ...string) Option {
	return func(o *Options) { o.Stop = seqs }
}

// WithModel 一次性覆盖模型名.
// [EN] Override model name for this call.
func WithModel(m string) Option {
	return func(o *Options) { o.Model = m }
}

// WithN 设置候选数.
// [EN] Set number of choices.
func WithN(n int) Option {
	return func(o *Options) { o.N = n }
}

// WithSeed 设置随机种子.
// [EN] Set random seed.
func WithSeed(seed int) Option {
	return func(o *Options) { o.Seed = seed }
}

// WithTimeout 设置单次请求超时.
// [EN] Set per-request timeout.
func WithTimeout(d time.Duration) Option {
	return func(o *Options) { o.Timeout = d }
}

// WithJSONMode 强制 JSON 输出.
// [EN] Force JSON output mode.
func WithJSONMode() Option {
	return func(o *Options) { o.JSONMode = true }
}

// WithUser 设置终端用户标识.
// [EN] Set end-user identifier.
func WithUser(u string) Option {
	return func(o *Options) { o.User = u }
}

// WithMaxToolIterations 设置工具调用循环上限（RunToolLoop 消费）.
// [EN] Set the tool loop limit (consumed by RunToolLoop).
func WithMaxToolIterations(n int) Option {
	return func(o *Options) { o.MaxToolIterations = n }
}

// Apply 应用选项到默认参数（核心与适配器共用的收口点）.
// [EN] Apply options onto defaults (shared entry for core and adapters).
func Apply(opts ...Option) *Options {
	o := DefaultOptions()
	for _, fn := range opts {
		if fn != nil {
			fn(o)
		}
	}
	return o
}
