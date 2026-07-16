/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 22:05:47
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 22:05:47
 * @FilePath: \go-llmx\textsplitter\token_bench_test.go
 * @Description: 近似 token 分块器基准 —— 流式字节扫描（现行）vs
 * []rune 全量复制 + 逐块 string 转换（优化前形态，对照组）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package textsplitter

import (
	"strings"
	"testing"
	"unicode"
)

// benchText 基准语料（中英混排，重复段拼至 ~100KB）.
// [EN] Benchmark corpus (mixed CJK/English, ~100KB).
func benchText() string {
	seg := "Go 语言在高并发场景下凭借 goroutine 与 channel 提供了轻量级的并发原语。 " +
		"The quick brown fox jumps over the lazy dog while measuring splitter throughput. "
	return strings.Repeat(seg, 900)
}

// BenchmarkTokenSplitter 流式字节扫描实现（现行）.
// [EN] The current streaming byte-scan implementation.
func BenchmarkTokenSplitter(b *testing.B) {
	text := benchText()
	s := NewTokenSplitter(512, 64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(s.Split(text)) == 0 {
			b.Fatal("no chunks")
		}
	}
}

// BenchmarkTokenSplitter_RuneCopy 对照组：优化前形态——
// []rune(text) 全量复制，每块 string(runes[a:b]) 二次转换.
// [EN] Control group: the pre-optimization shape.
func BenchmarkTokenSplitter_RuneCopy(b *testing.B) {
	text := benchText()
	s := NewTokenSplitter(512, 64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunks := splitRuneCopy(s, text)
		if len(chunks) == 0 {
			b.Fatal("no chunks")
		}
	}
}

// splitRuneCopy 优化前实现（对照组，逻辑与现行等价）.
// [EN] Pre-optimization implementation (control, equivalent logic).
func splitRuneCopy(s *TokenSplitter, text string) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	chunkRunes := s.chunkTokens * runesPerToken
	overlapRunes := s.overlapTokens * runesPerToken

	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + chunkRunes
		if end >= len(runes) {
			if chunk := strings.TrimRightFunc(string(runes[start:]), unicode.IsSpace); chunk != "" {
				chunks = append(chunks, chunk)
			}
			break
		}
		cut := end
		for i := end - 1; i > start+chunkRunes/2; i-- {
			if isBoundary(runes[i], runes[i-1]) {
				cut = i
				break
			}
		}
		if chunk := strings.TrimRightFunc(string(runes[start:cut]), unicode.IsSpace); chunk != "" {
			chunks = append(chunks, chunk)
		}
		start = cut - overlapRunes
		if start < cut-chunkRunes/2 {
			start = cut
		}
		if start <= 0 || start >= len(runes) {
			if start >= len(runes) {
				break
			}
			start = cut
		}
		for start > 0 && start < len(runes) && !isBoundary(runes[start], runes[start-1]) {
			start++
		}
	}
	return chunks
}
