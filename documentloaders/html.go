/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-16 13:15:00
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-16 22:15:16
 * @FilePath: \go-llmx\documentloaders\html.go
 * @Description: HTML 加载器 —— x/net/html 遍历 DOM 提取正文，
 * 跳过 script/style/head 等噪声节点，块级元素换行保留语义结构
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documentloaders

import (
	"context"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/textsplitter"
)

// skipTags 提取时跳过的噪声标签.
// [EN] Noise tags skipped during extraction.
var skipTags = map[string]struct{}{
	"script":   {},
	"style":    {},
	"head":     {},
	"noscript": {},
	"template": {},
}

// blockTags 块级标签（渲染前后补换行，保留结构）.
// [EN] Block tags (newline padding).
var blockTags = map[string]struct{}{
	"p": {}, "div": {}, "br": {}, "li": {}, "tr": {},
	"h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {},
	"section": {}, "article": {}, "header": {}, "footer": {},
	"blockquote": {}, "pre": {}, "table": {}, "ul": {}, "ol": {},
}

// HTML 文本加载器.
// [EN] HTML text loader.
type HTML struct {
	// reader 数据源.
	// [EN] Data source.
	reader io.Reader

	// source 源标识（URL 或文件名，拼入元数据）.
	// [EN] Source identifier (URL or filename).
	source string
}

// NewHTML 从读取器构造.
// [EN] Build from a reader.
func NewHTML(r io.Reader) *HTML {
	return &HTML{reader: r}
}

// WithSource 附加源标识.
// [EN] Attach a source identifier.
func (l *HTML) WithSource(source string) *HTML {
	l.source = source
	return l
}

// Load 实现 Loader（单文档，正文为提取文本）.
// [EN] Implement Loader (one document of extracted text).
func (l *HTML) Load(ctx context.Context) ([]llmx.Document, error) {
	root, err := html.Parse(l.reader)
	if err != nil {
		return nil, fmt.Errorf("html: %w", err)
	}

	var b strings.Builder
	walkText(root, &b)
	text := strings.TrimSpace(cleanBlankLines(b.String()))
	if text == "" {
		return nil, nil
	}

	meta := map[string]any{}
	if l.source != "" {
		meta["source"] = l.source
	}
	return []llmx.Document{{PageContent: text, Metadata: meta}}, nil
}

// LoadAndSplit 实现 Loader.
// [EN] Implement Loader.
func (l *HTML) LoadAndSplit(ctx context.Context, splitter textsplitter.Splitter) ([]llmx.Document, error) {
	return loadAndSplit(ctx, l, splitter)
}

// walkText 深度优先收集文本（跳过噪声标签；块级标签换行）.
// [EN] Depth-first text collection (skip noise; newline blocks).
func walkText(n *html.Node, b *strings.Builder) {
	if n.Type == html.ElementNode {
		if _, skip := skipTags[n.Data]; skip {
			return
		}
		if _, block := blockTags[n.Data]; block {
			b.WriteByte('\n')
		}
	}
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkText(c, b)
	}
	if n.Type == html.ElementNode {
		if _, block := blockTags[n.Data]; block {
			b.WriteByte('\n')
		}
	}
}

// cleanBlankLines 压缩连续空白行为单换行（保留结构不冗余）.
// [EN] Collapse blank runs.
func cleanBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			blank = true
			continue
		}
		if blank && len(out) > 0 {
			out = append(out, "")
		}
		blank = false
		out = append(out, strings.TrimSpace(ln))
	}
	return strings.Join(out, "\n")
}
