/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-22 19:59:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-22 20:07:19
 * @FilePath: \go-llmx\chain\stuff.go
 * @Description: Stuff 文档链 —— 全量文档拼接进单次请求（上下文窗口充裕时的首选，无中间轮次延迟最低）.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"
	"sync"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/prompt"
)

// Stuff 全量拼接文档链（一次模型调用）.
// [EN] Stuff chain (a single model call).
type Stuff struct {
	// llm 目标模型.
	// [EN] Target model.
	llm llmx.Model

	// tpl 提示词模板（nil 走 defaultStuffPrompt）.
	// [EN] Prompt template (nil uses the default).
	tpl *prompt.Template
}

// NewStuff 构造 Stuff 文档链.
// [EN] Build a Stuff chain.
func NewStuff(llm llmx.Model) *Stuff {
	return &Stuff{llm: llm}
}

// WithPrompt 覆盖默认模板（须包含 {{.Context}} 与 {{.Input}} 变量）.
// [EN] Override the default template.
func (c *Stuff) WithPrompt(tpl *prompt.Template) *Stuff {
	c.tpl = tpl
	return c
}

// Run 实现 DocumentChain（单次调用）.
// [EN] Implement DocumentChain (single call).
func (c *Stuff) Run(ctx context.Context, docs []llmx.Document, query string) (string, error) {
	if err := validateDocuments(docs, query); err != nil {
		return "", err
	}
	if c.llm == nil {
		return "", fmt.Errorf("%w: stuff chain missing model", llmx.ErrInvalidRequest)
	}
	text := joinDocuments(docs)
	rendered, err := renderDocumentPrompt(c.tpl, defaultStuffPrompt, documentVars{Context: text, Input: query})
	if err != nil {
		return "", err
	}
	resp, err := c.llm.GenerateContent(ctx, []llmx.Message{llmx.User(rendered)})
	if err != nil {
		return "", err
	}
	return llmx.FirstText(resp)
}

// defaultTemplates 缺省模板缓存：懒解析一次，后续 Run 命中缓存
// （langchaingo 每次 Run 重新 parse 模板，高频链路下纯浪费）.
// [EN] Default-template cache: parse once, hit afterwards.
var defaultTemplates sync.Map // string -> *prompt.Template

// defaultDocumentTemplate 取缺省模板（并发安全，解析一次）.
// [EN] Get a default template (parse once, thread-safe).
func defaultDocumentTemplate(fallback string) (*prompt.Template, error) {
	if v, ok := defaultTemplates.Load(fallback); ok {
		return v.(*prompt.Template), nil
	}
	compiled, err := prompt.New(fallback)
	if err != nil {
		return nil, err
	}
	actual, _ := defaultTemplates.LoadOrStore(fallback, compiled)
	return actual.(*prompt.Template), nil
}

// renderDocumentPrompt 渲染文档链提示词（nil 模板走缓存缺省）.
// [EN] Render a document-chain prompt (cached default when nil).
func renderDocumentPrompt(tpl *prompt.Template, fallback string, vars documentVars) (string, error) {
	if tpl == nil {
		compiled, err := defaultDocumentTemplate(fallback)
		if err != nil {
			return "", err
		}
		return compiled.Render(vars)
	}
	return tpl.Render(vars)
}

// compile-time assertion: DocumentChain 实现.
// [EN] Compile-time assertion.
var _ DocumentChain = (*Stuff)(nil)
