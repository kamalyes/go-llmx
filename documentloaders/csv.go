/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-29 21:20:31
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-29 21:27:01
 * @FilePath: \go-llmx\documentloaders\csv.go
 * @Description: CSV 加载器 —— 每行一文档，表头字段 → 元数据（标准库零依赖）.
 * 行内容序列化为 "列名: 值" 行集合，保留检索友好形态
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documentloaders

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/textsplitter"
)

// CSV 表格加载器（首行为表头）.
// [EN] CSV loader (first row as header).
type CSV struct {
	// reader 数据源（Load 时读取）.
	// [EN] Data source (read at Load time).
	reader io.Reader

	// source 源标识（拼入元数据，可选）.
	// [EN] Source identifier (into metadata, optional).
	source string
}

// NewCSV 从读取器构造.
// [EN] Build from a reader.
func NewCSV(r io.Reader) *CSV {
	return &CSV{reader: r}
}

// WithSource 附加源标识.
// [EN] Attach a source identifier.
func (l *CSV) WithSource(source string) *CSV {
	l.source = source
	return l
}

// Load 实现 Loader（每行一文档，表头字段 → 元数据）.
// [EN] Implement Loader (a document per row).
func (l *CSV) Load(ctx context.Context) ([]llmx.Document, error) {
	rows, err := csv.NewReader(l.reader).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	header := rows[0]
	var docs []llmx.Document
	for _, row := range rows[1:] {
		meta := map[string]any{}
		if l.source != "" {
			meta["source"] = l.source
		}
		var b strings.Builder
		for i, cell := range row {
			key := fmt.Sprintf("col_%d", i)
			if i < len(header) && header[i] != "" {
				key = header[i]
			}
			meta[key] = cell
			fmt.Fprintf(&b, "%s: %s\n", key, cell)
		}
		docs = append(docs, llmx.Document{
			PageContent: strings.TrimRight(b.String(), "\n"),
			Metadata:    meta,
		})
	}
	return docs, nil
}

// LoadAndSplit 实现 Loader.
// [EN] Implement Loader.
func (l *CSV) LoadAndSplit(ctx context.Context, splitter textsplitter.Splitter) ([]llmx.Document, error) {
	return loadAndSplit(ctx, l, splitter)
}
