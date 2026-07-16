/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 21:18:26
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 21:18:26
 * @FilePath: \go-llmx\textsplitter\token.go
 * @Description: 近似 token 分块器 —— 按 rune/4 估算 token 预算切分
 * （与 memory.ApproxTokens 同一规则），切点优先落在词边界，
 * 零依赖替代 langchaingo 依赖 tiktoken 的 TokenTextSplitter
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package textsplitter

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// runesPerToken 近似换算（英文 4 字符≈1 token 的行业通行估算）.
// [EN] Approximate conversion (4 chars ≈ 1 token).
const runesPerToken = 4

// TokenSplitter 近似 token 分块器（rune/4 估算，无需模型词表）.
// [EN] Approximate-token splitter (rune/4, no vocab needed).
type TokenSplitter struct {
	// chunkTokens 每块 token 上限（近似）.
	// [EN] Chunk token limit (approximate).
	chunkTokens int

	// overlapTokens 相邻块重叠 token 数.
	// [EN] Overlap between adjacent chunks.
	overlapTokens int
}

// NewTokenSplitter 构造（chunkTokens<=0 兜底 512；overlap 归一到块内）.
// [EN] Build (chunkTokens defaults to 512; overlap clamped).
func NewTokenSplitter(chunkTokens, overlapTokens int) *TokenSplitter {
	if chunkTokens <= 0 {
		chunkTokens = 512
	}
	if overlapTokens >= chunkTokens {
		overlapTokens = chunkTokens / 4
	}
	if overlapTokens < 0 {
		overlapTokens = 0
	}
	return &TokenSplitter{chunkTokens: chunkTokens, overlapTokens: overlapTokens}
}

// Split 实现 Splitter：按 token 预算滑窗切块，切点优先词边界.
// [EN] Implement Splitter: budget sliding window, word-boundary preferred.
func (s *TokenSplitter) Split(text string) []string {
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

		// 块内预算内寻找最后一个词边界（空白处），找不到才硬切
		// [EN] Find the last word boundary within budget; hard-cut otherwise.
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

		// 下一块起点：回退 overlap 个 rune，同样对齐词边界
		// [EN] Next start: rewind overlap runes, also boundary-aligned.
		start = cut - overlapRunes
		if start < cut-chunkRunes/2 { // overlap 超半块时防倒退
			start = cut
		}
		if start <= 0 || start >= len(runes) {
			if start >= len(runes) {
				break
			}
			start = cut
		}
		// 对齐词边界避免重叠切在词中间
		// [EN] Align to a boundary to avoid mid-word overlap.
		for start > 0 && start < len(runes) && !isBoundary(runes[start], runes[start-1]) {
			start++
		}
	}
	return chunks
}

// isBoundary 判断 r 前是否为可切词边界（当前是空白或前一个是空白）.
// [EN] Whether r is preceded by a word boundary.
func isBoundary(r, prev rune) bool {
	return unicode.IsSpace(r) || unicode.IsSpace(prev)
}

// ApproxTokens 近似 token 数（rune/4 向上取整），供预算估算复用.
// [EN] Approximate token count (rune/4, rounded up).
func ApproxTokens(text string) int {
	n := utf8.RuneCountInString(text)
	if n == 0 {
		return 0
	}
	return (n + runesPerToken - 1) / runesPerToken
}
