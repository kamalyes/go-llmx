/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-17 22:07:53
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-17 22:07:53
 * @FilePath: \go-llmx\retriever\multiquery.go
 * @Description: 多查询检索器 —— LLM 扩展查询变体 → 并行多路召回 → 有序去重合并.
 * 生成多个检索视角克服单一查询的相似度偏差（多路各取所长，原始查询恒为首路）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package retriever

import (
	"context"
	"strings"
	"sync"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/prompt"
)

// DefaultMultiQueryPrompt 查询扩展默认提示词（{{.question}} 为原始查询占位；
// 输出约定逐行一条、无编号无多余文本）.
// [EN] Default query-expansion prompt ({{.question}} placeholder; one
// phrasing per line, no numbering, no extra text).
const DefaultMultiQueryPrompt = `Generate 3 alternative phrasings of the user question to improve vector retrieval recall.
Output one phrasing per line, no numbering, no extra text.
User question: {{.question}}`

// MultiQueryRetriever 多查询检索器.
// [EN] Multi-query retriever.
type MultiQueryRetriever struct {
	// Ret 底层检索器（每路变体各召回一次）.
	// [EN] Underlying retriever (one call per variant).
	Ret llmx.Retriever

	// Model 查询扩展模型.
	// [EN] Model for query expansion.
	Model llmx.Model

	// Prompt 扩展提示词（缺省 DefaultMultiQueryPrompt；须含 {{.question}}）.
	// [EN] Expansion prompt (default when empty; must contain {{.question}}).
	Prompt string
}

// NewMultiQuery 构造多查询检索器.
// [EN] Build a multi-query retriever.
func NewMultiQuery(ret llmx.Retriever, model llmx.Model) *MultiQueryRetriever {
	return &MultiQueryRetriever{Ret: ret, Model: model}
}

// GetRelevantDocuments 实现 llmx.Retriever（扩展变体 → 并行召回 →
// 按查询路序合并去重，原始查询恒为首路；任一路失败整体失败）.
// [EN] Implement llmx.Retriever (expand variants → retrieve in parallel →
// merge and dedup in query order, original first; any failure fails all).
func (r *MultiQueryRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]llmx.Document, error) {
	if r.Ret == nil || r.Model == nil {
		return nil, llmx.ErrInvalidRequest
	}

	text := r.Prompt
	if text == "" {
		text = DefaultMultiQueryPrompt
	}
	tmpl, err := prompt.New(text)
	if err != nil {
		return nil, err
	}
	rendered, err := tmpl.Render(map[string]any{"question": query})
	if err != nil {
		return nil, err
	}
	out, err := llmx.Generate(ctx, r.Model, rendered)
	if err != nil {
		return nil, err
	}

	// 原查询恒为首路；模型输出无有效变体时退化为单路
	queries := append([]string{query}, parseQueryLines(out)...)
	results := make([][]llmx.Document, len(queries))
	errs := make([]error, len(queries))
	var wg sync.WaitGroup
	for i, q := range queries {
		wg.Add(1)
		go func(i int, q string) {
			defer wg.Done()
			results[i], errs[i] = r.Ret.GetRelevantDocuments(ctx, q)
		}(i, q)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	// 按路序合并去重（PageContent 为身份键，首见保留；与原查询重复的
	// 变体退化为一次冗余召回，由去重兜底）
	seen := make(map[string]struct{})
	var docs []llmx.Document
	for _, res := range results {
		for _, d := range res {
			if _, dup := seen[d.PageContent]; dup {
				continue
			}
			seen[d.PageContent] = struct{}{}
			docs = append(docs, d)
		}
	}
	return docs, nil
}

// parseQueryLines 解析模型输出为查询变体（逐行取非空行）.
// [EN] Parse model output into variants (non-empty lines).
func parseQueryLines(out string) []string {
	var variants []string
	for _, line := range strings.Split(out, "\n") {
		if v := strings.TrimSpace(line); v != "" {
			variants = append(variants, v)
		}
	}
	return variants
}

// 编译期断言：实现 llmx.Retriever 契约.
// [EN] Compile-time assertion.
var _ llmx.Retriever = (*MultiQueryRetriever)(nil)
