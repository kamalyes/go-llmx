/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-22 19:50:39
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-22 20:07:19
 * @FilePath: \go-llmx\chain\documents.go
 * @Description: 文档处理链契约 —— DocumentChain 接口与公共辅助.
 * 三种实现：Stuff（拼接单查）/ MapReduce（分治合并）/ Refine（迭代精炼）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// DocumentChain 文档处理链（多文档 + 查询 → 答案）.
// [EN] Document chain (documents + query to an answer).
type DocumentChain interface {
	// Run 处理文档并回答查询.
	// [EN] Process documents and answer the query.
	Run(ctx context.Context, docs []llmx.Document, query string) (string, error)
}

// 文档链模板变量（text/template 数据载体）.
// [EN] Document-chain template variables.
type documentVars struct {
	// Context 文档内容（拼接/分块形态由实现决定）.
	// [EN] Document content (shape decided by the implementation).
	Context string

	// Input 用户查询.
	// [EN] User query.
	Input string

	// Answer 前一轮答案（Refine 精炼链消费）.
	// [EN] Previous answer (consumed by Refine).
	Answer string
}

// 默认模板（stuff/mapreduce/refine 三链内置，可 WithPrompt 覆盖）.
// [EN] Default templates (overridable via WithPrompt).
const (
	defaultStuffPrompt = `Answer the question based only on the following context:

{{.Context}}

Question: {{.Input}}

Helpful Answer:`

	defaultMapPrompt = `The following is a set of summaries:

{{.Context}}

Take these and distill them into a final, consolidated summary of the main themes.
Helpful Answer:`

	defaultReducePrompt = `The following is a set of documents:

{{.Context}}

Based on this information, answer the question.
Question: {{.Input}}
Helpful Answer:`

	defaultRefinePrompt = `The original question is this: {{.Input}}

We have provided an existing answer: {{.Answer}}

We have the opportunity to refine the existing answer (only if needed) with some more context below.

------------
{{.Context}}
------------

Given the new context, refine the original answer to better answer the question.
If the context isn't useful, return the original answer.
Refined Answer:`
)

// joinDocuments 拼接文档为上下文块（空行分隔）.
// [EN] Join documents into a context block.
func joinDocuments(docs []llmx.Document) string {
	parts := make([]string, len(docs))
	for i, d := range docs {
		parts[i] = d.PageContent
	}
	return strings.Join(parts, "\n\n")
}

// validateDocuments 校验文档与查询非空（快速失败）.
// [EN] Validate non-empty documents and query.
func validateDocuments(docs []llmx.Document, query string) error {
	if len(docs) == 0 {
		return fmt.Errorf("%w: document chain requires at least one document", llmx.ErrInvalidRequest)
	}
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("%w: document chain requires a query", llmx.ErrInvalidRequest)
	}
	return nil
}
