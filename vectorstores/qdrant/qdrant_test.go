/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-14 21:33:08
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-14 22:05:19
 * @FilePath: \go-llmx\vectorstores\qdrant\qdrant_test.go
 * @Description: Qdrant 单测 —— httptest 模拟 REST 全端点：
 * 建集合/存量 info 维度对齐/批量写入 batch/检索嵌套过滤全路径覆盖，
 * 零 Qdrant 依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcqdrant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/transport"
)

// mockServer 模拟 Qdrant REST 端点.
// [EN] A mock Qdrant REST server.
type mockServer struct {
	// mu 保护可变状态.
	// [EN] Guards mutable state.
	mu sync.Mutex

	// exists 集合是否已存在（create 后置真）.
	// [EN] Whether the collection exists.
	exists bool

	// dim 集合维度.
	// [EN] Collection dimension.
	dim int

	// requests "METHOD 路径" → 请求体列表.
	// [EN] "METHOD path" to request bodies.
	requests map[string][]map[string]any

	// srv HTTP 服务.
	// [EN] The HTTP server.
	srv *httptest.Server
}

// newMockServer 启动模拟服务（exists 决定集合存量态）.
// [EN] Start the mock server (exists decides the initial state).
func newMockServer(t *testing.T, exists bool, dim int) *mockServer {
	t.Helper()
	m := &mockServer{
		exists:   exists,
		dim:      dim,
		requests: map[string][]map[string]any{},
	}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		m.mu.Lock()
		key := r.Method + " " + r.URL.Path
		m.requests[key] = append(m.requests[key], req)
		exists := m.exists
		dim := m.dim
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(path, pathCollections+"/"):
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"result":null,"status":"error"}`)
				return
			}
			fmt.Fprintf(w, `{"result":{"status":"green","config":{"params":{"vectors":{"size":%d,"distance":"Cosine"}}}},"status":"ok"}`, dim)
		case r.Method == http.MethodPut && path == pathCollections+"/"+"demo":
			m.mu.Lock()
			m.exists = true
			m.mu.Unlock()
			fmt.Fprint(w, `{"result":true,"status":"ok"}`)
		case r.Method == http.MethodPut && strings.Contains(path, pathPoints):
			fmt.Fprint(w, `{"result":{"operation_id":1,"status":"completed"},"status":"ok"}`)
		case r.Method == http.MethodPost && strings.HasSuffix(path, pathSearch):
			fmt.Fprint(w, `{"result":[
				{"id":"11111111-1111-4111-8111-111111111111","score":0.97,"payload":{"page_content":"hello","metadata":{"src":"wiki"}}},
				{"id":"22222222-2222-4222-8222-222222222222","score":0.91,"payload":{"page_content":"world"}}
			],"status":"ok","time":0.002}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"result":null,"status":"error"}`)
		}
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// store 基于模拟服务构造 Store.
// [EN] Build a Store against the mock server.
func (m *mockServer) store(opts ...Option) *Store {
	return newWithDoer(
		transport.NewClient(transport.WithHTTPClient(m.srv.Client())),
		m.srv.URL, opts...)
}

// bodyOf 取指定 "METHOD 路径" 第 idx 次请求体.
// [EN] Get the idx-th request body for a "METHOD path".
func (m *mockServer) bodyOf(key string, idx int) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	reqs := m.requests[key]
	if len(reqs) == 0 {
		return nil
	}
	if idx >= len(reqs) {
		idx = len(reqs) - 1
	}
	return reqs[idx]
}

// callsOf "METHOD 路径" 调用次数.
// [EN] Call count of a "METHOD path".
func (m *mockServer) callsOf(key string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests[key])
}

// collectionKey 集合资源请求键.
// [EN] Collection resource request key.
func collectionKey(method string) string {
	return method + " " + pathCollections + "/demo"
}

// pointsKey 写入资源请求键.
// [EN] Points resource request key.
func pointsKey() string {
	return http.MethodPut + " " + pathCollections + "/demo" + pathPoints
}

// searchKey 检索资源请求键.
// [EN] Search resource request key.
func searchKey() string {
	return http.MethodPost + " " + pathCollections + "/demo" + pathSearch
}

// TestInit_Create 验证全新建链路（404 → PUT 建集合）与载荷形态.
// [EN] Verify creation (404 → PUT) and the payload.
func TestInit_Create(t *testing.T) {
	m := newMockServer(t, false, 0)
	s := m.store(WithCollectionName("demo"), WithDimensions(8))

	require.NoError(t, s.Init(context.Background()))
	assert.Equal(t, 1, m.callsOf(collectionKey(http.MethodGet)))
	assert.Equal(t, 1, m.callsOf(collectionKey(http.MethodPut)))

	create := m.bodyOf(collectionKey(http.MethodPut), 0)
	assert.Equal(t, map[string]any{
		"vectors": map[string]any{"size": float64(8), "distance": "Cosine"},
	}, create)
}

// TestInit_CreateMissingDim 验证建集合缺维度报错.
// [EN] Verify the missing-dimension error on creation.
func TestInit_CreateMissingDim(t *testing.T) {
	m := newMockServer(t, false, 0)
	s := m.store(WithCollectionName("demo"))
	err := s.Init(context.Background())
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
	assert.Equal(t, 0, m.callsOf(collectionKey(http.MethodPut)))
}

// TestInit_MissingName 验证缺集合名报错.
// [EN] Verify the missing-collection-name error.
func TestInit_MissingName(t *testing.T) {
	m := newMockServer(t, true, 2)
	s := m.store()
	err := s.Init(context.Background())
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// TestInit_Existing 验证存量链路（info 对齐维度，不建集合）.
// [EN] Verify the existing path (info aligns the dimension).
func TestInit_Existing(t *testing.T) {
	m := newMockServer(t, true, 6)
	s := m.store(WithCollectionName("demo"))

	require.NoError(t, s.Init(context.Background()))
	assert.Equal(t, 1, m.callsOf(collectionKey(http.MethodGet)))
	assert.Equal(t, 0, m.callsOf(collectionKey(http.MethodPut)))
	assert.Equal(t, 6, s.dim)
}

// TestAddDocuments 验证批量写入载荷（UUIDv4 + 三列对齐 + wait）.
// [EN] Verify the batch upsert payload (UUIDv4 + aligned columns + wait).
func TestAddDocuments(t *testing.T) {
	m := newMockServer(t, true, 2)
	s := m.store(WithCollectionName("demo"), WithDimensions(2))
	require.NoError(t, s.Init(context.Background()))

	docs := []llmx.Document{
		{PageContent: "hello", Metadata: map[string]any{"src": "wiki"}},
		{PageContent: "world"},
	}
	require.NoError(t, s.AddDocuments(context.Background(), docs, [][]float64{{1, 0}, {0, 1}}))

	batch := m.bodyOf(pointsKey(), 0)["batch"].(map[string]any)
	ids := batch["ids"].([]any)
	require.Len(t, ids, 2)
	uuidRe := `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
	assert.Regexp(t, uuidRe, ids[0])

	vecs := batch["vectors"].([]any)
	assert.Equal(t, []any{float64(1), float64(0)}, vecs[0])

	payloads := batch["payloads"].([]any)
	p0 := payloads[0].(map[string]any)
	assert.Equal(t, "hello", p0["page_content"])
	assert.Equal(t, map[string]any{"src": "wiki"}, p0["metadata"])
	p1 := payloads[1].(map[string]any)
	assert.Equal(t, "world", p1["page_content"])
	assert.NotContains(t, p1, "metadata")
}

// TestAddDocuments_NoWait 验证吞吐优先免等待.
// [EN] Verify the throughput-first no-wait upsert.
func TestAddDocuments_NoWait(t *testing.T) {
	m := newMockServer(t, true, 2)
	s := m.store(WithCollectionName("demo"), WithDimensions(2), WithNoWait())
	require.NoError(t, s.Init(context.Background()))

	require.NoError(t, s.AddDocuments(context.Background(),
		[]llmx.Document{{PageContent: "a"}}, [][]float64{{1, 0}}))
	assert.Equal(t, 1, m.callsOf(pointsKey()))
}

// TestAddDocuments_Validation 验证数量与维度校验.
// [EN] Verify count and dimension validation.
func TestAddDocuments_Validation(t *testing.T) {
	s := newMockServer(t, true, 3).store(WithCollectionName("demo"), WithDimensions(3))

	err := s.AddDocuments(context.Background(),
		[]llmx.Document{{PageContent: "a"}},
		[][]float64{{1, 0}, {0, 1}})
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)

	err = s.AddDocuments(context.Background(),
		[]llmx.Document{{PageContent: "a"}},
		[][]float64{{1, 0}})
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)
}

// TestSimilaritySearch 验证检索载荷与命中还原.
// [EN] Verify the search payload and hit restoration.
func TestSimilaritySearch(t *testing.T) {
	m := newMockServer(t, true, 2)
	s := m.store(WithCollectionName("demo"), WithDimensions(2))
	require.NoError(t, s.Init(context.Background()))

	docs, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 5, llmx.Filter{"src": "wiki"})
	require.NoError(t, err)
	require.Len(t, docs, 2)
	assert.Equal(t, "hello", docs[0].PageContent)
	assert.Equal(t, "wiki", docs[0].Metadata["src"])
	assert.Equal(t, "world", docs[1].PageContent)
	assert.Nil(t, docs[1].Metadata)

	search := m.bodyOf(searchKey(), 0)
	assert.Equal(t, []any{float64(1), float64(0)}, search["vector"])
	assert.Equal(t, float64(5), search["limit"])
	assert.Equal(t, true, search["with_payload"])
	filter := search["filter"].(map[string]any)
	assert.Equal(t, []any{map[string]any{
		"key":   "metadata.src",
		"match": map[string]any{"value": "wiki"},
	}}, filter["must"])
}

// TestSimilaritySearch_DimMismatch 验证查询维度校验.
// [EN] Verify query dimension validation.
func TestSimilaritySearch_DimMismatch(t *testing.T) {
	s := newMockServer(t, true, 2).store(WithCollectionName("demo"), WithDimensions(2))
	_, err := s.SimilaritySearch(context.Background(), []float64{1, 0, 0}, 5)
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)
}

// TestBuildFilter 验证过滤构造（排序/多条件/类型透传）.
// [EN] Verify filter construction (ordering/multi/type passthrough).
func TestBuildFilter(t *testing.T) {
	assert.Nil(t, buildFilter(nil))

	f := buildFilter([]llmx.Filter{{"b": "y", "a": 1}, {"ok": true}})
	require.Len(t, f.Must, 3)
	assert.Equal(t, "metadata.a", f.Must[0].Key)
	assert.Equal(t, 1, f.Must[0].Match.Value)
	assert.Equal(t, "metadata.b", f.Must[1].Key)
	assert.Equal(t, "y", f.Must[1].Match.Value)
	assert.Equal(t, "metadata.ok", f.Must[2].Key)
	assert.Equal(t, true, f.Must[2].Match.Value)
}

// TestNewUUID 验证 UUIDv4 形态.
// [EN] Verify the UUIDv4 shape.
func TestNewUUID(t *testing.T) {
	uuidRe := `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
	assert.Regexp(t, uuidRe, newUUID())
	assert.NotEqual(t, newUUID(), newUUID())
}

// TestIsNotFound 验证 404 判定.
// [EN] Verify 404 detection.
func TestIsNotFound(t *testing.T) {
	assert.False(t, isNotFound(nil))
	assert.False(t, isNotFound(transport.NewStatusError(500, "boom")))
	assert.True(t, isNotFound(transport.NewStatusError(404, "missing")))
}
