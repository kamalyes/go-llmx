/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 20:26:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 20:26:00
 * @FilePath: \go-llmx\textsplitter\textsplitter.go
 * @Description: 递归分隔符分块器 —— RAG 索引前置处理.
 * 分隔符按优先级递归降级（\n\n → \n → 句读 → 空格 → 字符硬切），
 * 相邻块之间保留 overlap 衔接
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package textsplitter

import (
	"strings"
	"unicode/utf8"

	llmx "github.com/kamalyes/go-llmx"
)

// DefaultChunkSize 默认块大小（runes）.
// [EN] Default chunk size (in runes).
const DefaultChunkSize = 1000

// DefaultChunkOverlap 默认相邻块重叠（runes）.
// [EN] Default overlap between adjacent chunks (in runes).
const DefaultChunkOverlap = 200

// DefaultSeparators 默认递归分隔符（优先级从高到低；末位空串为字符级兜底）.
// [EN] Default recursive separators (high to low priority; trailing "" is the char-level fallback).
var DefaultSeparators = []string{"\n\n", "\n", "。", "．", ".", "！", "！", "；", ";", "？", "?", "，", ",", " ", ""}

// Splitter 文本分块器.
// [EN] Text splitter.
type Splitter interface {
	// Split 将文本切分为块.
	// [EN] Split text into chunks.
	Split(text string) []string
}

// RecursiveSplitter 递归分隔符分块器.
// [EN] Recursive-separator splitter.
type RecursiveSplitter struct {
	// chunkSize 块大小上限（runes）.
	// [EN] Chunk size limit (in runes).
	chunkSize int

	// chunkOverlap 相邻块重叠（runes）.
	// [EN] Overlap between adjacent chunks (in runes).
	chunkOverlap int

	// separators 递归分隔符.
	// [EN] Recursive separators.
	separators []string
}

// Option 构造选项.
// [EN] Constructor option.
type Option func(*RecursiveSplitter)

// WithChunkSize 设置块大小（runes；非法值由构造器归一）.
// [EN] Set chunk size (in runes; normalized by the constructor).
func WithChunkSize(n int) Option {
	return func(s *RecursiveSplitter) { s.chunkSize = n }
}

// WithChunkOverlap 设置相邻块重叠（runes；非法值由构造器归一）.
// [EN] Set chunk overlap (in runes; normalized by the constructor).
func WithChunkOverlap(n int) Option {
	return func(s *RecursiveSplitter) { s.chunkOverlap = n }
}

// WithSeparators 设置递归分隔符（空集回落默认；末位建议保留空串兜底）.
// [EN] Set recursive separators (empty set falls back to defaults).
func WithSeparators(seps ...string) Option {
	return func(s *RecursiveSplitter) { s.separators = seps }
}

// NewRecursiveSplitter 构造分块器（缺省 DefaultChunkSize/DefaultChunkOverlap/DefaultSeparators）.
// [EN] Build a splitter with defaults.
func NewRecursiveSplitter(opts ...Option) *RecursiveSplitter {
	s := &RecursiveSplitter{
		chunkSize:    DefaultChunkSize,
		chunkOverlap: DefaultChunkOverlap,
		separators:   DefaultSeparators,
	}
	for _, fn := range opts {
		if fn != nil {
			fn(s)
		}
	}
	if s.chunkSize <= 0 {
		s.chunkSize = DefaultChunkSize
	}
	if s.chunkOverlap < 0 || s.chunkOverlap >= s.chunkSize {
		s.chunkOverlap = 0
	}
	if len(s.separators) == 0 {
		s.separators = DefaultSeparators
	}
	return s
}

// Split 实现 Splitter（空文本返回 nil）.
// [EN] Implement Splitter (nil for empty text).
func (s *RecursiveSplitter) Split(text string) []string {
	if text == "" {
		return nil
	}
	pieces := s.splitRecursive(text, 0)
	return s.merge(pieces)
}

// splitRecursive 递归降级拆分：片段超限时以下一级分隔符继续拆.
// [EN] Recursive splitting: oversized pieces fall through to the next separator.
func (s *RecursiveSplitter) splitRecursive(text string, level int) []string {
	if utf8.RuneCountInString(text) <= s.chunkSize || level >= len(s.separators) {
		return []string{text}
	}
	sep := s.separators[level]
	if sep == "" {
		// 字符级兜底：硬切.
		// [EN] Char-level fallback: hard cut.
		return hardCut(text, s.chunkSize)
	}

	var out []string
	raw := strings.Split(text, sep)
	for i, piece := range raw {
		if i < len(raw)-1 {
			// 分隔符回填至本片段尾部，保持原文无损
			piece += sep
		}
		if piece == "" {
			continue
		}
		if utf8.RuneCountInString(piece) > s.chunkSize {
			out = append(out, s.splitRecursive(piece, level+1)...)
		} else {
			out = append(out, piece)
		}
	}
	return out
}

// merge 贪心合并相邻片段至块上限，块间以 overlap 尾部衔接.
// [EN] Greedily merge pieces up to the chunk limit, chaining chunks by overlap tails.
func (s *RecursiveSplitter) merge(pieces []string) []string {
	var chunks []string
	cur := ""
	for _, p := range pieces {
		if cur != "" && utf8.RuneCountInString(cur)+utf8.RuneCountInString(p) > s.chunkSize {
			chunks = append(chunks, cur)
			cur = runeTail(cur, s.chunkOverlap)
		}
		cur += p
	}
	if cur != "" {
		chunks = append(chunks, cur)
	}
	return chunks
}

// hardCut 按块大小字符级硬切.
// [EN] Hard-cut text at the chunk size.
func hardCut(text string, size int) []string {
	runes := []rune(text)
	out := make([]string, 0, (len(runes)+size-1)/size)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}

// runeTail 取字符串尾部 n 个 runes.
// [EN] Take the trailing n runes of a string.
func runeTail(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if n >= len(r) {
		return s
	}
	return string(r[len(r)-n:])
}

// SplitDocuments 分块并封装为 Document（元数据透传）.
// [EN] Split text and wrap chunks into Documents (metadata passthrough).
func SplitDocuments(s Splitter, text string, metadata map[string]any) []llmx.Document {
	chunks := s.Split(text)
	docs := make([]llmx.Document, 0, len(chunks))
	for _, c := range chunks {
		docs = append(docs, llmx.Document{PageContent: c, Metadata: metadata})
	}
	return docs
}
