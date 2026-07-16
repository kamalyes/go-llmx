/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 21:18:26
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 21:58:03
 * @FilePath: \go-llmx\textsplitter\token.go
 * @Description: 近似 token 分块器 —— 按 rune/4 估算 token 预算切分
 * （与 memory.ApproxTokens 同一规则），切点优先落在词边界；
 * 流式字节扫描实现（utf8 原地解码 + 字节切片切块零复制），
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
// 流式字节扫描：块直接 text[byteA:byteB] 切片共享底层，无 []rune 复制
// 与逐块 string 转换（对照 langchaingo tiktoken 路径的词表解码开销）.
// [EN] Implement Splitter: budget sliding window, word-boundary preferred.
// Streaming byte scan: chunks are zero-copy subslices of the source.
func (s *TokenSplitter) Split(text string) []string {
	if text == "" {
		return nil
	}

	chunkRunes := s.chunkTokens * runesPerToken
	overlapRunes := s.overlapTokens * runesPerToken

	var chunks []string
	bStart := 0
	for bStart < len(text) {
		// 前向扫描一个窗口：满 chunkRunes 个 rune 或文本耗尽
		// [EN] Scan one window: chunkRunes runes or end of text.
		bPos, rCount := bStart, 0
		bEnd, bBound := -1, -1 // 窗口末字节、半窗后最后词边界字节
		var prevR rune
		for bPos < len(text) {
			r, size := utf8.DecodeRuneInString(text[bPos:])
			rCount++
			if rCount > chunkRunes/2 && isBoundary(r, prevR) {
				bBound = bPos // 持续覆盖 → 窗口内最后一个边界
			}
			prevR = r
			bPos += size
			if rCount >= chunkRunes {
				bEnd = bPos
				break
			}
		}

		// 尾块：文本耗尽（rCount < chunkRunes）
		// [EN] Tail chunk: text exhausted.
		if bEnd < 0 {
			if chunk := strings.TrimRightFunc(text[bStart:], unicode.IsSpace); chunk != "" {
				chunks = append(chunks, chunk)
			}
			break
		}

		cut := bEnd
		if bBound > bStart {
			cut = bBound // 词边界切块，找不到（无空白的连续长文）硬切
		}
		if chunk := strings.TrimRightFunc(text[bStart:cut], unicode.IsSpace); chunk != "" {
			chunks = append(chunks, chunk)
		}

		// overlap 回退：从 cut 反向走 overlapRunes 个 rune（窗口小，回退廉价）
		// [EN] Overlap rewind: step back overlapRunes runes from cut.
		nStart := cut
		if overlapRunes > 0 {
			nStart = rewindRunes(text, cut, overlapRunes)
			if half := bStart + (cut-bStart)/2; nStart < half {
				nStart = cut // 过度回退防倒退：不跨过半窗
			}
			// 前向对齐词边界，避免重叠切在词中间
			// [EN] Align forward to a word boundary.
			for nStart < len(text) && nStart > bStart {
				r, size := utf8.DecodeRuneInString(text[nStart:])
				pr, _ := utf8.DecodeLastRuneInString(text[:nStart])
				if isBoundary(r, pr) {
					break
				}
				nStart += size
			}
		}
		if nStart <= bStart {
			nStart = cut // 保进度：绝不原地打转
		}
		bStart = nStart
	}
	return chunks
}

// rewindRunes 从字节位置 pos 反向回退 n 个 rune（不越界越过 floor）.
// [EN] Step back n runes from byte position pos.
func rewindRunes(text string, pos, n int) int {
	for i := 0; i < n && pos > 0; i++ {
		_, size := utf8.DecodeLastRuneInString(text[:pos])
		pos -= size
	}
	return pos
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
