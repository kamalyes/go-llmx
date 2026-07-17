/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-17 21:52:08
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-17 21:52:08
 * @FilePath: \go-llmx\retriever\parent.go
 * @Description: 父文档检索器 —— 小块索引精准召回，返回完整父文档.
 * 索引：全文存 DocStore、切分子块（携带 parent_id）入 VectorStore；
 * 检索：子块相似召回 → 回溯去重父文档 —— 兼顾小块嵌入精度与完整上下文
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package retriever

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync/atomic"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/docstore"
	"github.com/kamalyes/go-llmx/textsplitter"
)

// MetaKeyParentID 子块元数据中的父文档键（AddDocuments 写入，检索时回溯）.
// [EN] Parent-doc key in child chunk metadata (written on index, followed on retrieval).
const MetaKeyParentID = "parent_id"

// docSeq 父文档键序列（进程内单调递增，确定性便于测试与排障）.
// [EN] Parent-doc key sequence (monotonic in-process, deterministic for tests).
var docSeq atomic.Int64

// ParentDocumentRetriever 父文档检索器.
// [EN] Parent-document retriever.
type ParentDocumentRetriever struct {
	// VS 小块向量库.
	// [EN] Chunk vector store.
	VS llmx.VectorStore

	// Emb 小块/查询嵌入器.
	// [EN] Chunk/query embedder.
	Emb llmx.Embedder

	// Store 父文档键值存储.
	// [EN] Parent-doc store.
	Store docstore.DocStore

	// Splitter 索引期切分器.
	// [EN] Indexing-time splitter.
	Splitter textsplitter.Splitter

	// K 子块召回条数（<=0 走 llmx.DefaultRetrievalK）.
	// [EN] Child recall count.
	K int
}

// NewParentDocument 构造父文档检索器.
// [EN] Build a parent-document retriever.
func NewParentDocument(vs llmx.VectorStore, emb llmx.Embedder, store docstore.DocStore, splitter textsplitter.Splitter, k int) *ParentDocumentRetriever {
	return &ParentDocumentRetriever{VS: vs, Emb: emb, Store: store, Splitter: splitter, K: k}
}

// AddDocuments 索引文档：全文入 DocStore，子块（继承父元数据 + parent_id）
// 单批嵌入后入 VectorStore（空内容文档跳过）.
// [EN] Index documents: full docs into the DocStore, child chunks (parent
// metadata + parent_id) embedded once as a batch into the VectorStore
// (empty docs skipped).
func (r *ParentDocumentRetriever) AddDocuments(ctx context.Context, docs []llmx.Document) error {
	if r.VS == nil || r.Emb == nil || r.Store == nil || r.Splitter == nil {
		return llmx.ErrInvalidRequest
	}

	var chunks []string
	var children []llmx.Document
	for _, doc := range docs {
		if doc.PageContent == "" {
			continue
		}
		id := fmt.Sprintf("doc-%09d", docSeq.Add(1))
		if err := r.Store.Set(ctx, id, doc); err != nil {
			return err
		}
		for _, chunk := range r.Splitter.Split(doc.PageContent) {
			meta := make(map[string]any, len(doc.Metadata)+1)
			maps.Copy(meta, doc.Metadata)
			meta[MetaKeyParentID] = id
			chunks = append(chunks, chunk)
			children = append(children, llmx.Document{PageContent: chunk, Metadata: meta})
		}
	}
	if len(chunks) == 0 {
		return nil
	}

	vectors, err := r.Emb.EmbedDocuments(ctx, chunks)
	if err != nil {
		return err
	}
	if len(vectors) != len(chunks) {
		return fmt.Errorf("%w: %d vectors for %d chunks", llmx.ErrInvalidVectors, len(vectors), len(chunks))
	}
	return r.VS.AddDocuments(ctx, children, vectors)
}

// GetRelevantDocuments 实现 llmx.Retriever（子块召回 → parent_id 回溯 →
// 去重父文档保序返回；缺 parent_id 或父文档已删的子块跳过）.
// [EN] Implement llmx.Retriever (child recall → parent lookup → deduped
// parents in order; children without linkage or with deleted parents skipped).
func (r *ParentDocumentRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]llmx.Document, error) {
	if r.VS == nil || r.Emb == nil || r.Store == nil {
		return nil, llmx.ErrInvalidRequest
	}

	k := r.K
	if k <= 0 {
		k = llmx.DefaultRetrievalK
	}
	vec, err := r.Emb.EmbedQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	children, err := r.VS.SimilaritySearch(ctx, vec, k)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(children))
	parents := make([]llmx.Document, 0, len(children))
	for _, child := range children {
		id, ok := child.Metadata[MetaKeyParentID].(string)
		if !ok || id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		parent, err := r.Store.Get(ctx, id)
		if errors.Is(err, docstore.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		seen[id] = struct{}{}
		parents = append(parents, parent)
	}
	return parents, nil
}

// 编译期断言：实现 llmx.Retriever 契约.
// [EN] Compile-time assertion.
var _ llmx.Retriever = (*ParentDocumentRetriever)(nil)
