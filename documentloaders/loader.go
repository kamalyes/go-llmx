/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-29 20:07:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-29 20:13:55
 * @FilePath: \go-llmx\documentloaders\loader.go
 * @Description: 文档加载契约 —— Loader 接口与 LoadAndSplit 公共骨架.
 * 零依赖实现：text/csv/directory；PDF/HTML 等重型格式由适配器扩展
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documentloaders

import (
	"context"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/textsplitter"
)

// Loader 文档加载器.
// [EN] Document loader.
type Loader interface {
	// Load 加载全部文档.
	// [EN] Load all documents.
	Load(ctx context.Context) ([]llmx.Document, error)

	// LoadAndSplit 加载并按分块器切分.
	// [EN] Load and split with a splitter.
	LoadAndSplit(ctx context.Context, splitter textsplitter.Splitter) ([]llmx.Document, error)
}

// loadAndSplit 公共骨架：先 Load 全量，再对每文档分块（元数据继承）.
// [EN] Shared skeleton: load first, then split per document (metadata inherited).
func loadAndSplit(ctx context.Context, l Loader, splitter textsplitter.Splitter) ([]llmx.Document, error) {
	docs, err := l.Load(ctx)
	if err != nil {
		return nil, err
	}
	if splitter == nil {
		return docs, nil
	}
	var out []llmx.Document
	for _, doc := range docs {
		for _, chunk := range splitter.Split(doc.PageContent) {
			out = append(out, llmx.Document{
				PageContent: chunk,
				Metadata:    doc.Metadata,
			})
		}
	}
	return out, nil
}
