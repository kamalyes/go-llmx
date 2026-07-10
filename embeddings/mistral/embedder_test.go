/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-10 21:26:18
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-10 21:26:18
 * @FilePath: \go-llmx\embeddings\mistral\embedder_test.go
 * @Description: Mistral 嵌入适配器测试 —— 批量/单条协议、index 归位、
 * 数量不齐哨兵、错误映射、认证头
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmembed

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

// embedMock Mistral /embeddings 协议模拟端点.
// [EN] Mistral /embeddings protocol mock.
func embedMock(t *testing.T, mismatch bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"message":"bad key","type":"unauthorized","code":401}}`)
			return
		}

		var req wireRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		n := len(req.Input)
		if mismatch {
			n-- // 少回一条触发数量哨兵
		}
		data := make([]wirePayload, n)
		for i := range data {
			data[i] = wirePayload{Index: i, Embedding: []float64{float64(i), 1}}
		}
		_ = json.NewEncoder(w).Encode(wireResponse{Data: data})
	}))
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

func TestEmbedQuery_Single(t *testing.T) {
	srv := embedMock(t, false)
	defer srv.Close()
	c := New("test-key", WithBaseURL(srv.URL))

	vec, err := c.EmbedQuery(context.Background(), "查询文本")
	require.NoError(t, err)
	assert.Equal(t, []float64{0, 1}, vec)
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
