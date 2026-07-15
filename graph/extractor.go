/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-15 20:51:32
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-15 21:03:47
 * @FilePath: \go-llmx\graph\extractor.go
 * @Description: 三元组提取器 —— TripleExtractor 契约与 LLM 实现，
 * 输出解析 "主体|谓词|客体" 行式协议，容错跳过畸形行
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package graph

import (
	"context"
	"fmt"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// extractorPrompt 三元组提取指令（行式协议，容错解析）.
// [EN] Extraction instruction (line protocol, tolerant parsing).
const extractorPrompt = `从下面的文本中抽取知识三元组，每行一条，严格遵循格式：
主体|谓词|客体
不要输出其他内容；没有可抽取的三元组时输出空文本。

文本：
%s`

// TripleExtractor 三元组提取器（文本 → 三元组）.
// [EN] Triple extractor (text → triples).
type TripleExtractor interface {
	// Extract 从文本抽取三元组.
	// [EN] Extract triples from text.
	Extract(ctx context.Context, text string) ([]Triple, error)
}

// LLMExtractor 基于 LLM 的提取器（行式协议解析）.
// [EN] LLM-backed extractor (line-protocol parsing).
type LLMExtractor struct {
	// model 对话模型.
	// [EN] Chat model.
	model llmx.Model
}

// NewLLMExtractor 构造 LLM 提取器.
// [EN] Build an LLM extractor.
func NewLLMExtractor(model llmx.Model) *LLMExtractor {
	return &LLMExtractor{model: model}
}

// Extract 实现 TripleExtractor.
// [EN] Implement TripleExtractor.
func (e *LLMExtractor) Extract(ctx context.Context, text string) ([]Triple, error) {
	out, err := llmx.Generate(ctx, e.model, fmt.Sprintf(extractorPrompt, text))
	if err != nil {
		return nil, err
	}
	return ParseTriples(out), nil
}

// ParseTriples 解析行式协议文本（"主体|谓词|客体"，空行/畸形行跳过，去重）.
// [EN] Parse line-protocol text (dedup; blank/malformed lines skipped).
func ParseTriples(text string) []Triple {
	var triples []Triple
	seen := map[string]struct{}{}
	for _, line := range strings.Split(text, "\n") {
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) != 3 {
			continue
		}
		t := Triple{
			Subject:   strings.TrimSpace(parts[0]),
			Predicate: strings.TrimSpace(parts[1]),
			Object:    strings.TrimSpace(parts[2]),
		}
		if t.Subject == "" || t.Predicate == "" || t.Object == "" {
			continue
		}
		k := t.key()
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		triples = append(triples, t)
	}
	return triples
}
