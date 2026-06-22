/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-22 22:39:26
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-22 22:51:26
 * @FilePath: \go-llmx\chain\refine.go
 * @Description: Refine 文档链 —— 逐文档迭代精炼（携带前一轮答案吸收新上下文）.
 * 适合答案随上下文逐步演进的场景；N 文档 N 次调用，无中间摘要丢失
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/prompt"
)

// Refine 迭代精炼文档链.
// [EN] Iterative refine chain.
type Refine struct {
	// llm 目标模型.
	// [EN] Target model.
	llm llmx.Model

	// tpl 精炼模板（nil 走 defaultRefinePrompt；变量 Context/Input/Answer）.
	// [EN] Refine template.
	tpl *prompt.Template
}

// NewRefine 构造 Refine 文档链.
// [EN] Build a Refine chain.
func NewRefine(llm llmx.Model) *Refine {
	return &Refine{llm: llm}
}

// WithPrompt 覆盖精炼模板.
// [EN] Override the refine template.
func (c *Refine) WithPrompt(tpl *prompt.Template) *Refine {
	c.tpl = tpl
	return c
}

// Run 实现 DocumentChain（首轮以查询直答，后续逐文档精炼）.
// [EN] Implement DocumentChain (direct answer first, then refine per document).
func (c *Refine) Run(ctx context.Context, docs []llmx.Document, query string) (string, error) {
	if err := validateDocuments(docs, query); err != nil {
		return "", err
	}
	if c.llm == nil {
		return "", fmt.Errorf("%w: refine chain missing model", llmx.ErrInvalidRequest)
	}

	// 首轮：查询直答（携带首文档）
	answer, err := c.answer(ctx, docs[0], query, "")
	if err != nil {
		return "", fmt.Errorf("refine stage 0: %w", err)
	}

	// 后续：逐文档精炼（Answer 携带前一轮结果）
	for i := 1; i < len(docs); i++ {
		answer, err = c.answer(ctx, docs[i], query, answer)
		if err != nil {
			return "", fmt.Errorf("refine stage %d: %w", i, err)
		}
	}
	return answer, nil
}

// answer 单轮精炼调用（Answer 为空时模板变量渲染为空串）.
// [EN] One refine round (an empty Answer renders as "").
func (c *Refine) answer(ctx context.Context, doc llmx.Document, query, previous string) (string, error) {
	rendered, err := renderDocumentPrompt(c.tpl, defaultRefinePrompt, documentVars{
		Context: doc.PageContent,
		Input:   query,
		Answer:  previous,
	})
	if err != nil {
		return "", err
	}
	resp, err := c.llm.GenerateContent(ctx, []llmx.Message{llmx.User(rendered)})
	if err != nil {
		return "", err
	}
	return llmx.FirstText(resp)
}

// compile-time assertion: DocumentChain 实现.
// [EN] Compile-time assertion.
var _ DocumentChain = (*Refine)(nil)
