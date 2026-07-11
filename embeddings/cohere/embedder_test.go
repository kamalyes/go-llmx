/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-11 21:06:43
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-11 21:06:43
 * @FilePath: \go-llmx\embeddings\cohere\embedder_test.go
 * @Description: Cohere 嵌入适配器测试 —— input_type 非对称语义、批量/单条协议、
 * 数量不齐哨兵、错误映射、认证头
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccembed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// embedMock Cohere /v2/embed 协议模拟端点.
// [EN] Cohere /v2/embed protocol mock.
func embedMock(t *testing.T, mismatch bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"unauthorized"}`)
			return
		}

		var req wireRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, DefaultModel, req.Model)
		require.Equal(t, []string{embeddingTypesFloats}, req.EmbeddingTypes)

		n := len(req.Texts)
		if mismatch {
			n--
		}
		vectors := make([][]float64, n)
		for i := range vectors {
			vectors[i] = []float64{float64(i), 1}
		}
		_ = json.NewEncoder(w).Encode(wireResponse{Embeddings: wireEmbeddingSet{Float: vectors}})
	}))
}

func TestEmbedDocuments_InputTypeSemantics(t *testing.T) {
	// 索引阶段：input_type=search_document
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req wireRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		got = req.InputType
		_ = json.NewEncoder(w).Encode(wireResponse{Embeddings: wireEmbeddingSet{Float: [][]float64{{0, 1}}}})
	}))
	defer srv.Close()
	c := New("test-key", WithBaseURL(srv.URL))

	vectors, err := c.EmbedDocuments(context.Background(), []string{"文本一"})
	require.NoError(t, err)
	assert.Equal(t, [][]float64{{0, 1}}, vectors)
	assert.Equal(t, inputTypeDocument, got)
}

func TestEmbedQuery_InputTypeSemantics(t *testing.T) {
	// 查询阶段：input_type=search_query
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req wireRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		got = req.InputType
		_ = json.NewEncoder(w).Encode(wireResponse{Embeddings: wireEmbeddingSet{Float: [][]float64{{0.5, 0.5}}}})
	}))
	defer srv.Close()
	c := New("test-key", WithBaseURL(srv.URL))

	vec, err := c.EmbedQuery(context.Background(), "查询文本")
	require.NoError(t, err)
	assert.Equal(t, []float64{0.5, 0.5}, vec)
	assert.Equal(t, inputTypeQuery, got)
}

func TestEmbedDocuments_Batch(t *testing.T) {
	srv := embedMock(t, false)
	defer srv.Close()
	c := New("test-key", WithBaseURL(srv.URL))

	vectors, err := c.EmbedDocuments(context.Background(), []string{"文本一", "文本二"})
	require.NoError(t, err)
	require.Len(t, vectors, 2)
	assert.Equal(t, []float64{0, 1}, vectors[0])
	assert.Equal(t, []float64{1, 1}, vectors[1])
}

func TestEmbed_CountMismatch(t *testing.T) {
	srv := embedMock(t, true)
	defer srv.Close()
	c := New("test-key", WithBaseURL(srv.URL))

	_, err := c.EmbedDocuments(context.Background(), []string{"文本一", "文本二"})
	require.ErrorIs(t, err, llmx.ErrEmptyResponse)
}

func TestEmbed_Unauthorized(t *testing.T) {
	srv := embedMock(t, false)
	defer srv.Close()
	c := New("wrong-key", WithBaseURL(srv.URL))

	_, err := c.EmbedQuery(context.Background(), "查询")
	require.ErrorIs(t, err, llmx.ErrUnauthorized)
}

func TestEmbed_EmptyInput(t *testing.T) {
	c := New("test-key")
	_, err := c.EmbedDocuments(context.Background(), nil)
	require.Error(t, err)
}
