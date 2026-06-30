/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-29 20:06:53
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-30 22:38:51
 * @FilePath: \go-llmx\documentloaders\text.go
 * @Description: 文本加载器 —— string / io.Reader → 单文档.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documentloaders

import (
	"bytes"
	"context"
	"io"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/textsplitter"
)

// Text 文本加载器.
// [EN] Text loader.
type Text struct {
	// content 文本内容（NewText/NewTextReader 二选一构造）.
	// [EN] Text content.
	content string

	// metadata 附加元数据（可选）.
	// [EN] Optional metadata.
	metadata map[string]any
}

// NewText 从字符串构造.
// [EN] Build from a string.
func NewText(content string) *Text {
	return &Text{content: content}
}

// NewTextReader 从读取器构造（构造期全量读入，Load 时无 IO）.
// bytes.Buffer 承接后零拷贝转 string，避免 ReadAll 后的二次全量拷贝.
// [EN] Build from a reader (fully read at construction; Buffer.String transfers without an extra copy).
func NewTextReader(r io.Reader) (*Text, error) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}
	return &Text{content: buf.String()}, nil
}

// WithMetadata 附加元数据（源标识等）.
// [EN] Attach metadata (source identifiers, etc).
func (l *Text) WithMetadata(metadata map[string]any) *Text {
	l.metadata = metadata
	return l
}

// Load 实现 Loader（单文档）.
// [EN] Implement Loader (a single document).
func (l *Text) Load(ctx context.Context) ([]llmx.Document, error) {
	return []llmx.Document{{
		PageContent: l.content,
		Metadata:    l.metadata,
	}}, nil
}

// LoadAndSplit 实现 Loader.
// [EN] Implement Loader.
func (l *Text) LoadAndSplit(ctx context.Context, splitter textsplitter.Splitter) ([]llmx.Document, error) {
	return loadAndSplit(ctx, l, splitter)
}
