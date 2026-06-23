/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-23 22:29:39
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-23 22:37:58
 * @FilePath: \go-llmx\retriever_test.go
 * @Description: 检索器测试 —— 嵌入/Top-K 传递/默认条数/参数校验.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubEmbedder 固定向量嵌入.
// [EN] Fixed-vector embedder.
type stubEmbedder struct{ vec []float64 }

func (e stubEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = e.vec
	}
	return out, nil
}

func (e stubEmbedder) EmbedQuery(ctx context.Context, text string) ([]float64, error) {
	return e.vec, nil
}

// stubVectorStore 记录调用的向量与 topK.
// [EN] Vector store recording the query vector and topK.
type stubVectorStore struct {
	gotVec []float64
	gotK   int
	docs   []Document
	err    error
}

func (s *stubVectorStore) AddDocuments(ctx context.Context, docs []Document, vectors [][]float64) error {
	return nil
}

func (s *stubVectorStore) SimilaritySearch(ctx context.Context, query []float64, topK int, filters ...Filter) ([]Document, error) {
	s.gotVec, s.gotK = query, topK
	return s.docs, s.err
}

func TestRetriever_TopKPassThrough(t *testing.T) {
	vs := &stubVectorStore{docs: []Document{{PageContent: "d1"}}}
	r := NewRetriever(vs, stubEmbedder{vec: []float64{1, 0}}, 7)

	docs, err := r.GetRelevantDocuments(context.Background(), "q")
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, 7, vs.gotK)
	assert.Equal(t, []float64{1, 0}, vs.gotVec)
}

func TestRetriever_DefaultK(t *testing.T) {
	vs := &stubVectorStore{}
	r := NewRetriever(vs, stubEmbedder{vec: []float64{0.5}}, 0)

	_, err := r.GetRelevantDocuments(context.Background(), "q")
	require.NoError(t, err)
	assert.Equal(t, DefaultRetrievalK, vs.gotK)
}

func TestRetriever_StoreErrorPropagates(t *testing.T) {
	vs := &stubVectorStore{err: errors.New("store down")}
	r := NewRetriever(vs, stubEmbedder{vec: []float64{1}}, 3)

	_, err := r.GetRelevantDocuments(context.Background(), "q")
	require.ErrorContains(t, err, "store down")
}

func TestRetriever_MissingDeps(t *testing.T) {
	r := NewRetriever(nil, nil, 3)
	_, err := r.GetRelevantDocuments(context.Background(), "q")
	require.ErrorIs(t, err, ErrInvalidRequest)
}
