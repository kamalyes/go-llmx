/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-16 20:15:00
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-16 23:15:56
 * @FilePath: \go-llmx\documentloaders\json.go
 * @Description: JSON 加载器 —— 顶层数组或点路径定位的数组每元素一文档，
 * 字段提取零依赖（标准库 encoding/json + 点路径导航）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documentloaders

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/textsplitter"
)

// JSON 数组加载器（每元素一文档）.
// [EN] JSON array loader (a document per element).
type JSON struct {
	// reader 数据源.
	// [EN] Data source.
	reader io.Reader

	// path 点路径定位数组（空串取顶层数组，如 "items.data"）.
	// [EN] Dot path to the array ("" = top level).
	path string

	// contentKey 元素内容字段（空串取元素整体序列化）.
	// [EN] Element content field ("" = whole element).
	contentKey string

	// source 源标识.
	// [EN] Source identifier.
	source string
}

// NewJSON 从读取器构造.
// [EN] Build from a reader.
func NewJSON(r io.Reader) *JSON {
	return &JSON{reader: r}
}

// WithPath 点路径定位数组（如 "items.data"；空串取顶层数组）.
// [EN] Dot path to the array (e.g. "items.data").
func (l *JSON) WithPath(path string) *JSON {
	l.path = path
	return l
}

// WithContentKey 元素内容字段名（默认整体序列化）.
// [EN] Content field of each element.
func (l *JSON) WithContentKey(key string) *JSON {
	l.contentKey = key
	return l
}

// WithSource 附加源标识.
// [EN] Attach a source identifier.
func (l *JSON) WithSource(source string) *JSON {
	l.source = source
	return l
}

// Load 实现 Loader（数组每元素一文档）.
// [EN] Implement Loader (a document per element).
func (l *JSON) Load(ctx context.Context) ([]llmx.Document, error) {
	raw, err := io.ReadAll(l.reader)
	if err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}

	// 点路径导航到目标节点
	// [EN] Navigate the dot path to the target node.
	node := root
	if l.path != "" {
		for _, seg := range strings.Split(l.path, ".") {
			obj, ok := node.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("json: path segment %q not an object", seg)
			}
			node, ok = obj[seg]
			if !ok {
				return nil, fmt.Errorf("json: path segment %q missing", seg)
			}
		}
	}

	items, ok := node.([]any)
	if !ok {
		return nil, fmt.Errorf("json: path %q is not an array", l.path)
	}

	docs := make([]llmx.Document, 0, len(items))
	for i, item := range items {
		content := ""
		meta := map[string]any{"seq": i}
		if l.source != "" {
			meta["source"] = l.source
		}
		if key := l.contentKey; key != "" {
			obj, isMap := item.(map[string]any)
			if !isMap {
				return nil, fmt.Errorf("json: element %d not an object for content key", i)
			}
			val, exists := obj[key]
			if !exists {
				return nil, fmt.Errorf("json: element %d missing content key %q", i, key)
			}
			text, isStr := val.(string)
			if !isStr {
				serialized, err := json.Marshal(val)
				if err != nil {
					return nil, err
				}
				text = string(serialized)
			}
			content = text
			for k, v := range obj {
				if k != key {
					meta[k] = v
				}
			}
		} else if text, isStr := item.(string); isStr {
			content = text // 纯字符串元素直取，不序列化带引号
			// [EN] Plain string elements pass through unquoted.
		} else {
			serialized, err := json.Marshal(item)
			if err != nil {
				return nil, err
			}
			content = string(serialized)
		}
		docs = append(docs, llmx.Document{PageContent: content, Metadata: meta})
	}
	return docs, nil
}

// LoadAndSplit 实现 Loader.
// [EN] Implement Loader.
func (l *JSON) LoadAndSplit(ctx context.Context, splitter textsplitter.Splitter) ([]llmx.Document, error) {
	return loadAndSplit(ctx, l, splitter)
}
