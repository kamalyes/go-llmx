/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-29 21:20:31
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-30 22:38:51
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
	"strconv"
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
	docs := make([]llmx.Document, 0, len(rows)-1)
	for _, row := range rows[1:] {
		meta := make(map[string]any, len(row)+1)
		if l.source != "" {
			meta["source"] = l.source
		}
		var b strings.Builder
		b.Grow(64 * len(row))
		for i, cell := range row {
			var key string
			if i < len(header) {
				key = header[i]
			}
			if key == "" {
				key = "col_" + strconv.Itoa(i)
			}
			meta[key] = cell
			// 手写拼接替代 Fprintf：省反射格式化与中间分配
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(cell)
			b.WriteByte('\n')
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
