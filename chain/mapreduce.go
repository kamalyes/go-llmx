/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-22 21:27:53
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-22 21:33:52
 * @FilePath: \go-llmx\chain\mapreduce.go
 * @Description: MapReduce 文档链 —— 每文档独立摘要（map），合并摘要作答（reduce）.
 * 适合文档总量超出上下文窗口的场景，代价是 1+N 次模型调用
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/prompt"
)

// MapReduce 分治文档链（逐文档摘要 → 合并作答）.
// [EN] MapReduce chain (per-document summary, then a combined answer).
type MapReduce struct {
	// llm 目标模型（map 与 reduce 复用）.
	// [EN] Target model (shared by map and reduce).
	llm llmx.Model

	// mapTpl 摘要模板（nil 走 defaultMapPrompt）.
	// [EN] Summary template.
	mapTpl *prompt.Template

	// reduceTpl 合并作答模板（nil 走 defaultReducePrompt）.
	// [EN] Combined-answer template.
	reduceTpl *prompt.Template
}

// NewMapReduce 构造 MapReduce 文档链.
// [EN] Build a MapReduce chain.
func NewMapReduce(llm llmx.Model) *MapReduce {
	return &MapReduce{llm: llm}
}

// WithMapPrompt 覆盖摘要模板（变量 {{.Context}}）.
// [EN] Override the summary template.
func (c *MapReduce) WithMapPrompt(tpl *prompt.Template) *MapReduce {
	c.mapTpl = tpl
	return c
}

// WithReducePrompt 覆盖合并模板（变量 {{.Context}} 与 {{.Input}}）.
// [EN] Override the reduce template.
func (c *MapReduce) WithReducePrompt(tpl *prompt.Template) *MapReduce {
	c.reduceTpl = tpl
	return c
}

// Run 实现 DocumentChain（map 阶段逐文档摘要，reduce 阶段合并作答）.
// [EN] Implement DocumentChain (map per document, then reduce).
func (c *MapReduce) Run(ctx context.Context, docs []llmx.Document, query string) (string, error) {
	if err := validateDocuments(docs, query); err != nil {
		return "", err
	}
	if c.llm == nil {
		return "", fmt.Errorf("%w: mapreduce chain missing model", llmx.ErrInvalidRequest)
	}

	// map 阶段：逐文档摘要
	summaries := make([]string, len(docs))
	for i, doc := range docs {
		rendered, err := renderDocumentPrompt(c.mapTpl, defaultMapPrompt, documentVars{Context: doc.PageContent})
		if err != nil {
			return "", fmt.Errorf("map stage doc %d: %w", i, err)
		}
		resp, err := c.llm.GenerateContent(ctx, []llmx.Message{llmx.User(rendered)})
		if err != nil {
			return "", fmt.Errorf("map stage doc %d: %w", i, err)
		}
		summary, err := llmx.FirstText(resp)
		if err != nil {
			return "", fmt.Errorf("map stage doc %d: %w", i, err)
		}
		summaries[i] = summary
	}

	// reduce 阶段：合并摘要作答
	rendered, err := renderDocumentPrompt(c.reduceTpl, defaultReducePrompt, documentVars{
		Context: strings.Join(summaries, "\n\n"),
		Input:   query,
	})
	if err != nil {
		return "", fmt.Errorf("reduce stage: %w", err)
	}
	resp, err := c.llm.GenerateContent(ctx, []llmx.Message{llmx.User(rendered)})
	if err != nil {
		return "", fmt.Errorf("reduce stage: %w", err)
	}
	return llmx.FirstText(resp)
}

// compile-time assertion: DocumentChain 实现.
// [EN] Compile-time assertion.
var _ DocumentChain = (*MapReduce)(nil)
