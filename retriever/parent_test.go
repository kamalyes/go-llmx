/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-17 22:21:36
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-17 22:21:36
 * @FilePath: \go-llmx\retriever\parent_test.go
 * @Description: 父文档检索器测试 —— 索引链路（全文入库/子块挂链/单批嵌入）/
 * 检索链路（父文档去重保序/缺链跳过/已删跳过）/参数校验.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package retriever

import (
	"context"
	"strings"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/docstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// stub 基建
// ============================================================================

// stubEmbedder 固定向量嵌入器（记录批量/单条调用次数）.
// [EN] Fixed-vector embedder (counts batch/query calls).
type stubEmbedder struct {
	mu         sync.Mutex
	batchCalls int
	queryCalls int
	vec        []float64
	err        error
}

func (e *stubEmbedder) EmbedDocuments(_ context.Context, texts []string) ([][]float64, error) {
	e.mu.Lock()
	e.batchCalls++
	e.mu.Unlock()
	if e.err != nil {
		return nil, e.err
	}
	out := make([][]float64, len(texts))
	for i := range out {
		out[i] = e.vec
	}
	return out, nil
}

func (e *stubEmbedder) EmbedQuery(_ context.Context, text string) ([]float64, error) {
	e.mu.Lock()
	e.queryCalls++
	e.mu.Unlock()
	return e.vec, e.err
}

// stubVectorStore 记录写入与召回参数的向量库.
// [EN] Vector store recording writes and recall parameters.
type stubVectorStore struct {
	mu            sync.Mutex
	addedDocs     []llmx.Document
	addedVectors  [][]float64
	hitChildren   []llmx.Document
	gotK          int
	searchQueries int
	err           error
}

func (s *stubVectorStore) AddDocuments(_ context.Context, docs []llmx.Document, vectors [][]float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addedDocs = append(s.addedDocs, docs...)
	s.addedVectors = append(s.addedVectors, vectors...)
	return nil
}

func (s *stubVectorStore) SimilaritySearch(_ context.Context, query []float64, topK int, _ ...llmx.Filter) ([]llmx.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gotK = topK
	s.searchQueries++
	return s.hitChildren, s.err
}

// lineSplitter 按行切分（确定性子块）.
// [EN] Line-based splitter (deterministic chunks).
type lineSplitter struct{}

func (lineSplitter) Split(text string) []string { return strings.Split(text, "\n") }

// ============================================================================
// 索引链路
// ============================================================================

func TestParent_AddDocuments_LinksAndBatches(t *testing.T) {
	vs, emb := &stubVectorStore{}, &stubEmbedder{vec: []float64{1}}
	store := docstore.NewInMemory()
	r := NewParentDocument(vs, emb, store, lineSplitter{}, 2)

	docs := []llmx.Document{
		{PageContent: "A1\nA2", Metadata: map[string]any{"src": "a.md"}},
		{PageContent: "B1\nB2", Metadata: map[string]any{"src": "b.md"}},
	}
	require.NoError(t, r.AddDocuments(context.Background(), docs))

	// 全文入 DocStore
	keys := store.Keys(context.Background())
	require.Len(t, keys, 2)
	for _, k := range keys {
		got, err := store.Get(context.Background(), k)
		require.NoError(t, err)
		assert.Contains(t, []string{"A1\nA2", "B1\nB2"}, got.PageContent)
	}

	// 子块四块两条父链，元数据继承 + parent_id
	require.Len(t, vs.addedDocs, 4)
	ids := map[string]bool{}
	for _, c := range vs.addedDocs {
		id := c.Metadata[MetaKeyParentID].(string)
		ids[id] = true
		assert.NotEmpty(t, c.Metadata["src"])
	}
	require.Len(t, ids, 2)

	// 单批嵌入
	assert.Equal(t, 1, emb.batchCalls)
	require.Len(t, vs.addedVectors, 4)
}

func TestParent_AddDocuments_EmptyDocSkipped(t *testing.T) {
	vs, emb := &stubVectorStore{}, &stubEmbedder{vec: []float64{1}}
	store := docstore.NewInMemory()
	r := NewParentDocument(vs, emb, store, lineSplitter{}, 2)

	require.NoError(t, r.AddDocuments(context.Background(), []llmx.Document{{PageContent: ""}}))

	assert.Empty(t, store.Keys(context.Background()))
	assert.Zero(t, emb.batchCalls)
	assert.Empty(t, vs.addedDocs)
}

func TestParent_AddDocuments_NilDeps(t *testing.T) {
	r := &ParentDocumentRetriever{}
	err := r.AddDocuments(context.Background(), []llmx.Document{{PageContent: "x"}})
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// ============================================================================
// 检索链路
// ============================================================================

func TestParent_Retrieval_ReturnsDedupedParents(t *testing.T) {
	store := docstore.NewInMemory()
	ctx := context.Background()
	require.NoError(t, store.Set(ctx, "p1", llmx.Document{PageContent: "full-1"}))
	require.NoError(t, store.Set(ctx, "p2", llmx.Document{PageContent: "full-2"}))

	vs := &stubVectorStore{hitChildren: []llmx.Document{
		{PageContent: "c1", Metadata: map[string]any{MetaKeyParentID: "p1"}},
		{PageContent: "c2", Metadata: map[string]any{MetaKeyParentID: "p1"}},
		{PageContent: "c3", Metadata: map[string]any{MetaKeyParentID: "p2"}},
	}}
	r := NewParentDocument(vs, &stubEmbedder{vec: []float64{1}}, store, lineSplitter{}, 5)

	docs, err := r.GetRelevantDocuments(ctx, "q")
	require.NoError(t, err)

	// 命中三块返回两父（p1 去重），保序
	require.Len(t, docs, 2)
	assert.Equal(t, "full-1", docs[0].PageContent)
	assert.Equal(t, "full-2", docs[1].PageContent)
	assert.Equal(t, 5, vs.gotK)
}

func TestParent_Retrieval_UnlinkedAndMissingSkipped(t *testing.T) {
	store := docstore.NewInMemory()
	ctx := context.Background()
	require.NoError(t, store.Set(ctx, "p1", llmx.Document{PageContent: "full-1"}))

	vs := &stubVectorStore{hitChildren: []llmx.Document{
		{PageContent: "no-meta"},
		{PageContent: "ghost", Metadata: map[string]any{MetaKeyParentID: "deleted"}},
		{PageContent: "c1", Metadata: map[string]any{MetaKeyParentID: "p1"}},
	}}
	r := NewParentDocument(vs, &stubEmbedder{vec: []float64{1}}, store, lineSplitter{}, 3)

	docs, err := r.GetRelevantDocuments(ctx, "q")
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, "full-1", docs[0].PageContent)
}

func TestParent_Retrieval_KDefaults(t *testing.T) {
	vs := &stubVectorStore{}
	r := NewParentDocument(vs, &stubEmbedder{vec: []float64{1}}, docstore.NewInMemory(), lineSplitter{}, 0)

	_, err := r.GetRelevantDocuments(context.Background(), "q")
	require.NoError(t, err)
	assert.Equal(t, llmx.DefaultRetrievalK, vs.gotK)
}

func TestParent_Retrieval_NilDeps(t *testing.T) {
	r := &ParentDocumentRetriever{}
	_, err := r.GetRelevantDocuments(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}
