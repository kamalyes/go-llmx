/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-08-11 20:58:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-08-11 20:58:00
 * @FilePath: \go-llmx\adapter\client.go
 * @Description: 适配器公共基座 —— Client 字段/选项/访问器/请求组装骨架.
 * openai/anthropic 等对话适配器内嵌 Base 复用全部客户端管理能力，
 * 仅保留协议差异（wire 编解码/stream 聚合/认证头/错误类型映射）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package adapter

import (
	"net/http"
	"strings"
	"time"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/transport"
)

// Base 适配器客户端基座（apiKey/端点/模型/传输的公共管理）.
// [EN] Adapter client base (shared apiKey/endpoint/model/transport management).
type Base struct {
	// APIKey 认证密钥（认证头形态由各适配器构造）.
	// [EN] Auth key (header style is adapter-specific).
	APIKey string

	// BaseURL 端点（尾斜杠已归一，兼容网关/代理）.
	// [EN] Endpoint (trailing slash normalized, gateway compatible).
	BaseURL string

	// Model 默认模型.
	// [EN] Default model.
	Model string

	// Path 协议路径（ChatCompletionsPath / MessagesPath 等）.
	// [EN] Protocol path (ChatCompletionsPath / MessagesPath, etc).
	Path string

	// TC 公共传输客户端（JSON/SSE 收口）.
	// [EN] Shared transport client.
	TC *transport.Client
}

// NewBase 构造基座（path 必填；传输取默认；BaseURL 由适配器填默认端点）.
// [EN] Build a base (path required; transport defaulted; BaseURL set by the adapter).
func NewBase(path, apiKey, defaultModel string) Base {
	b := Base{
		APIKey: apiKey,
		Model:  defaultModel,
		Path:   path,
	}
	b.TC = transport.NewClient()
	return b
}

// HasBase 基座持有者（适配器 Client 内嵌 Base 并实现本接口，供多态选项定位；
// 导出以支持适配器定义本地选项，如 openai.WithEmbedModel）.
// [EN] Base holder (adapters embed Base and implement this for polymorphic options;
// exported so adapters can define local options, e.g. openai.WithEmbedModel).
type HasBase interface {
	Adapter() *Base
}

// Option 适配器构造选项（多态作用于任意内嵌 Base 的适配器客户端）.
// [EN] Adapter constructor option (polymorphic over any Base-embedding client).
type Option func(HasBase)

// WithAPIKey 设置 API Key.
// [EN] Set the API key.
func WithAPIKey(key string) Option {
	return func(h HasBase) { h.Adapter().APIKey = key }
}

// WithBaseURL 设置端点（兼容网关/代理；尾斜杠归一）.
// [EN] Set the endpoint (gateway compatible; trailing slash normalized).
func WithBaseURL(url string) Option {
	return func(h HasBase) { h.Adapter().SetBaseURL(url) }
}

// WithModel 设置默认模型.
// [EN] Set the default model.
func WithModel(m string) Option {
	return func(h HasBase) { h.Adapter().SetModel(m) }
}

// WithTimeout 设置请求总超时.
// [EN] Set the total request timeout.
func WithTimeout(d time.Duration) Option {
	return func(h HasBase) {
		h.Adapter().TC = transport.NewClient(transport.WithTimeout(d))
	}
}

// WithHTTPClient 注入自定义 http.Client（代理/连接池调优；nil 忽略）.
// [EN] Inject a custom http.Client (nil ignored).
func WithHTTPClient(hc *http.Client) Option {
	return func(h HasBase) {
		if hc != nil {
			h.Adapter().TC = transport.NewClient(transport.WithHTTPClient(hc))
		}
	}
}

// Apply 应用选项到任意内嵌 Base 的客户端（各适配器 New 的统一收口）.
// [EN] Apply options to any Base-embedding client (unified adapter New exit).
func Apply(c HasBase, opts ...Option) {
	for _, fn := range opts {
		if fn != nil {
			fn(c)
		}
	}
}

// ============================================================================
// 访问器（Get/Set 双通道：构造期 With 定型，运行期 Set 热更）
// ============================================================================

// GetBaseURL 返回当前端点.
// [EN] Return the current endpoint.
func (b *Base) GetBaseURL() string { return b.BaseURL }

// SetBaseURL 运行期切换端点（multi-region 灾备 / 灰度网关；尾斜杠归一）.
// [EN] Switch the endpoint at runtime (trailing slash normalized).
func (b *Base) SetBaseURL(url string) { b.BaseURL = strings.TrimRight(url, "/") }

// GetModel 返回默认模型.
// [EN] Return the default model.
func (b *Base) GetModel() string { return b.Model }

// SetModel 运行期切换默认模型（模型下线/成本调整时热更；空串不生效防误清空）.
// [EN] Switch the default model at runtime (empty ignored).
func (b *Base) SetModel(m string) {
	if m != "" {
		b.Model = m
	}
}

// GetAPIKey 返回认证密钥.
// [EN] Return the API key.
func (b *Base) GetAPIKey() string { return b.APIKey }

// SetAPIKey 运行期轮换密钥（密钥泄露应急 / 定期轮换策略）.
// [EN] Rotate the API key at runtime.
func (b *Base) SetAPIKey(key string) { b.APIKey = key }

// GetEndpoint 返回完整协议 URL（baseURL + path 收口拼接）.
// [EN] Return the full protocol URL.
func (b *Base) GetEndpoint() string { return b.BaseURL + b.Path }

// ResolveModel 解析本次实际模型（请求级覆盖 > 客户端默认）.
// [EN] Resolve the effective model (per-call override wins).
func (b *Base) ResolveModel(o *llmx.Options) string {
	if o != nil && o.Model != "" {
		return o.Model
	}
	return b.Model
}
