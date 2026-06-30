/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-30 20:58:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-30 22:38:51
 * @FilePath: \go-llmx\textsplitter\markdown.go
 * @Description: Markdown 感知分块器 —— 标题边界优先、代码块不切断，
 * 超长节内部递归退化到 RecursiveSplitter（零依赖自实现，不引第三方解析器）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package textsplitter

import (
	"strings"
	"unicode/utf8"
)

// 代码围栏标记.
// [EN] Code fence marker.
const codeFence = "```"

// Markdown Markdown 感知分块器.
// [EN] Markdown-aware splitter.
type Markdown struct {
	// inner 超长节内部退化分块（复用 RecursiveSplitter）.
	// [EN] Inner fallback splitter (reuses RecursiveSplitter).
	inner *RecursiveSplitter
}

// NewMarkdownSplitter 构造 Markdown 分块器（块参数透传内部 RecursiveSplitter）.
// [EN] Build a markdown splitter (chunk options passed to the inner splitter).
func NewMarkdownSplitter(opts ...Option) *Markdown {
	return &Markdown{inner: NewRecursiveSplitter(opts...)}
}

// Split 实现 Splitter（结构边界优先，超长节二次切分）.
// [EN] Implement Splitter (structure first, then size fallback).
func (m *Markdown) Split(text string) []string {
	sections := splitSections(text)
	if len(sections) == 0 {
		return m.inner.Split(text)
	}

	var chunks []string
	for _, sec := range sections {
		if m.inner.chunkSize > 0 && utf8.RuneCountInString(sec) <= m.inner.chunkSize {
			chunks = append(chunks, sec)
			continue
		}
		// 超长节：内部退化为递归分块（代码围栏可能被切断，属可接受代价）
		chunks = append(chunks, m.inner.Split(sec)...)
	}

	// 跳过切分后产生的空块
	out := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if strings.TrimSpace(c) != "" {
			out = append(out, c)
		}
	}
	return out
}

// splitSections 按标题与代码块边界切节（保持节内原文，含边界行）.
// 行扫描与节切片均为原串视图，零中间分配.
// [EN] Split into sections at heading and code-fence boundaries.
// Both line scan and section slices are views over the original string (zero intermediate allocs).
func splitSections(text string) []string {
	var sections []string
	start := 0   // 当前节起始偏移
	lastEnd := 0 // 已消费最后一个非换行偏移
	hasContent := false
	inCode := false

	for pos := 0; pos < len(text); {
		lineStart := pos
		nl := strings.IndexByte(text[pos:], '\n')
		var lineEnd int
		if nl < 0 {
			lineEnd = len(text)
			pos = len(text)
		} else {
			lineEnd = pos + nl
			pos = lineEnd + 1
		}

		trimmed := strings.TrimSpace(text[lineStart:lineEnd])
		switch {
		case strings.HasPrefix(trimmed, codeFence):
			// 代码围栏状态跟踪（围栏内不切）
			inCode = !inCode
		case !inCode && isHeading(trimmed) && hasContent:
			// 标题行（# 开头，围栏外）为节边界：先落上一节，标题归新节
			sections = append(sections, text[start:lastEnd])
			start = lineStart
		}
		hasContent = true
		lastEnd = lineEnd
	}
	if hasContent {
		sections = append(sections, text[start:lastEnd])
	}
	return sections
}

// isHeading 判定 Markdown 标题行（# ~ ######）.
// [EN] Heading predicate (# to ######).
func isHeading(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	return level <= 6 && level < len(trimmed) && trimmed[level] == ' '
}
