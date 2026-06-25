/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-25 20:10:28
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-25 20:16:55
 * @FilePath: \go-llmx\chain\retrievalqa.go
 * @Description: RetrievalQA 检索问答链 —— Retriever 检索 + 拼接上下文 + 模型作答，
 * RAG 的最小编排闭环
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

// defaultRetrievalQAPrompt 检索问答默认模板.
// [EN] Default retrieval-QA template.
const defaultRetrievalQAPrompt = `Answer the question based only on the following context:

{{.Context}}

Question: {{.Input}}

Helpful Answer:`

// RetrievalQA 检索问答链.
// [EN] Retrieval question-answering chain.
type RetrievalQA struct {
	// Model 目标模型.
	// [EN] Target model.
	Model llmx.Model

	// Retriever 检索器.
	// [EN] Retriever.
	Retriever llmx.Retriever

	// tpl 提示词模板（nil 走默认）.
	// [EN] Prompt template (nil uses the default).
	tpl *prompt.Template
}

// NewRetrievalQA 构造检索问答链.
// [EN] Build a retrieval-QA chain.
func NewRetrievalQA(model llmx.Model, retriever llmx.Retriever) *RetrievalQA {
	return &RetrievalQA{Model: model, Retriever: retriever}
}

// WithPrompt 覆盖问答模板（变量 {{.Context}} 与 {{.Input}}）.
// [EN] Override the QA template.
func (c *RetrievalQA) WithPrompt(tpl *prompt.Template) *RetrievalQA {
	c.tpl = tpl
	return c
}

// Run 检索并作答（无检索结果时如实告知，不虚构）.
// [EN] Retrieve then answer (states when nothing is found).
func (c *RetrievalQA) Run(ctx context.Context, query string) (string, error) {
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("%w: retrieval QA requires a query", llmx.ErrInvalidRequest)
	}
	if c.Model == nil || c.Retriever == nil {
		return "", fmt.Errorf("%w: retrieval QA requires a model and a retriever", llmx.ErrInvalidRequest)
	}
	docs, err := c.Retriever.GetRelevantDocuments(ctx, query)
	if err != nil {
		return "", fmt.Errorf("retrieve: %w", err)
	}
	if len(docs) == 0 {
		return "I don't have enough context to answer this question.", nil
	}
	rendered, err := renderDocumentPrompt(c.tpl, defaultRetrievalQAPrompt, documentVars{
		Context: joinDocuments(docs),
		Input:   query,
	})
	if err != nil {
		return "", err
	}
	resp, err := c.Model.GenerateContent(ctx, []llmx.Message{llmx.User(rendered)})
	if err != nil {
		return "", err
	}
	return llmx.FirstText(resp)
}
