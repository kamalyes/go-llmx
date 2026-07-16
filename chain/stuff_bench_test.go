/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 17:12:36
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 17:12:36
 * @FilePath: \go-llmx\chain\stuff_bench_test.go
 * @Description: 缺省模板渲染基准 —— 缓存命中 vs 逐次解析
 * （优化前 renderDocumentPrompt 每次 prompt.New，优化后 sync.Map 命中）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/prompt"
)

// benchDocs 基准用文档集（20 篇 x 200 字符）.
// [EN] Benchmark documents (20 docs x 200 chars).
func benchDocs() []llmx.Document {
	docs := make([]llmx.Document, 20)
	for i := range docs {
		content := make([]byte, 200)
		for j := range content {
			content[j] = byte('a' + j%26)
		}
		docs[i] = llmx.Document{PageContent: string(content)}
	}
	return docs
}

// BenchmarkRenderDocumentPrompt_Cached 缺省模板渲染（缓存命中路径）.
// [EN] Default-prompt rendering (cache-hit path).
func BenchmarkRenderDocumentPrompt_Cached(b *testing.B) {
	vars := documentVars{Context: joinDocuments(benchDocs()), Input: "summarize"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := renderDocumentPrompt(nil, defaultStuffPrompt, vars); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRenderDocumentPrompt_NoCache 对照组：模拟优化前逐次解析
// （langchaingo 每次 Run 都走这条路径）.
// [EN] Control group: per-call parsing as before the fix.
func BenchmarkRenderDocumentPrompt_NoCache(b *testing.B) {
	vars := documentVars{Context: joinDocuments(benchDocs()), Input: "summarize"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled, err := prompt.New(defaultStuffPrompt)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := compiled.Render(vars); err != nil {
			b.Fatal(err)
		}
	}
}
