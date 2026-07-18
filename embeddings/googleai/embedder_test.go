/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-09 21:16:52
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-18 10:27:11
 * @FilePath: \go-llmx\embeddings\googleai\embedder_test.go
 * @Description: Google AI 嵌入适配器测试 —— 批量/单条协议、taskType 语义、
 * 超限自动分批（有界并行）、错误状态映射、认证头
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgembed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// embedMock 按方法分发的嵌入协议模拟端点.
// [EN] Method-dispatching embedding protocol mock.
func embedMock(t *testing.T, batchHook func(n int)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"code":401,"status":"UNAUTHENTICATED","message":"bad key"}}`)
			return
		}

		switch {
		case strings.HasSuffix(r.URL.Path, ":batchEmbedContents"):
			var req wireBatchRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			if batchHook != nil {
				batchHook(len(req.Requests))
			}
			// 校验索引阶段 taskType 语义
			for _, rr := range req.Requests {
				require.Equal(t, taskTypeDocument, rr.TaskType)
			}
			embeddings := make([]wireValues, len(req.Requests))
			for i := range req.Requests {
				embeddings[i] = wireValues{Values: []float64{float64(i), 1}}
			}
			_ = json.NewEncoder(w).Encode(wireBatchResponse{Embeddings: embeddings})

		case strings.HasSuffix(r.URL.Path, ":embedContent"):
			var req wireEmbedRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.Equal(t, taskTypeQuery, req.TaskType)
			_ = json.NewEncoder(w).Encode(wireSingleResponse{Embedding: wireValues{Values: []float64{0.5, 0.5}}})

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":{"code":404,"status":"NOT_FOUND","message":"unknown method"}}`)
		}
	}))
}

func TestEmbedDocuments_Batch(t *testing.T) {
	srv := embedMock(t, nil)
	defer srv.Close()
	c := New("test-key", WithBaseURL(srv.URL))

	vectors, err := c.EmbedDocuments(context.Background(), []string{"文本一", "文本二", "文本三"})
	require.NoError(t, err)
	require.Len(t, vectors, 3)
	// 响应顺序与输入一致
	assert.Equal(t, []float64{0, 1}, vectors[0])
	assert.Equal(t, []float64{1, 1}, vectors[1])
	assert.Equal(t, []float64{2, 1}, vectors[2])
}

func TestEmbedDocuments_AutoBatching(t *testing.T) {
	// 并行批 hook 收集需互斥；完成序不定，排序后断言批次大小
	var (
		mu         sync.Mutex
		batchSizes []int
	)
	srv := embedMock(t, func(n int) {
		mu.Lock()
		batchSizes = append(batchSizes, n)
		mu.Unlock()
	})
	defer srv.Close()
	c := New("test-key", WithBaseURL(srv.URL))

	// 250 条 → 100 + 100 + 50 三批（有界并行）
	texts := make([]string, 250)
	for i := range texts {
		texts[i] = fmt.Sprintf("文本%d", i)
	}
	vectors, err := c.EmbedDocuments(context.Background(), texts)
	require.NoError(t, err)
	require.Len(t, vectors, 250)

	mu.Lock()
	sizes := append([]int(nil), batchSizes...)
	mu.Unlock()
	sort.Ints(sizes)
	assert.Equal(t, []int{50, 100, 100}, sizes)
}

func TestEmbedQuery_Single(t *testing.T) {
	srv := embedMock(t, nil)
	defer srv.Close()
	c := New("test-key", WithBaseURL(srv.URL))

	vec, err := c.EmbedQuery(context.Background(), "查询文本")
	require.NoError(t, err)
	assert.Equal(t, []float64{0.5, 0.5}, vec)
}

func TestEmbed_Unauthorized(t *testing.T) {
	srv := embedMock(t, nil)
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
