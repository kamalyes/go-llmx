/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-13 22:51:07
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-13 23:07:26
 * @FilePath: \go-llmx\vectorstores\milvus\milvus_test.go
 * @Description: Milvus 单测 —— httptest 模拟 RESTful v2 全端点：
 * 建集合 schema/存量 describe/写入与刷盘/检索过滤表达式全路径覆盖，
 * 零 Milvus 依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmilvus

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

// mockServer 模拟 RESTful v2 端点.
// [EN] A mock RESTful v2 server.
type mockServer struct {
	// mu 保护可变状态.
	// [EN] Guards mutable state.
	mu sync.Mutex

	// exists 集合是否已存在（create 后置真）.
	// [EN] Whether the collection exists.
	exists bool

	// loadState describe 返回的加载态.
	// [EN] Load state returned by describe.
	loadState string

	// requests 路径 → 请求体列表.
	// [EN] Path to request bodies.
	requests map[string][]map[string]any

	// srv HTTP 服务.
	// [EN] The HTTP server.
	srv *httptest.Server
}

// newMockServer 启动模拟服务.
// [EN] Start the mock server.
func newMockServer(t *testing.T, exists bool, loadState string) *mockServer {
	t.Helper()
	m := &mockServer{
		exists:    exists,
		loadState: loadState,
		requests:  map[string][]map[string]any{},
	}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		m.mu.Lock()
		path := strings.TrimPrefix(r.URL.Path, apiVersion)
		m.requests[path] = append(m.requests[path], req)
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch path {
		case pathCollectionsHas:
			m.mu.Lock()
			has := m.exists
			m.mu.Unlock()
			fmt.Fprintf(w, `{"code":0,"data":{"has":%t}}`, has)
		case pathCollectionsCreate:
			m.mu.Lock()
			m.exists = true
			m.mu.Unlock()
			fmt.Fprint(w, `{"code":0,"message":"success"}`)
		case pathCollectionsDescr:
			fmt.Fprintf(w, `{"code":0,"data":{"load":%q,"fields":[
				{"fieldName":"pk","dataType":"Int64","isPrimary":true},
				{"fieldName":"text","dataType":"VarChar"},
				{"fieldName":"meta","dataType":"JSON"},
				{"fieldName":"vector","dataType":"FloatVector","elementTypeParams":{"dim":"2"}}
			]}}`, m.loadState)
		case pathCollectionsLoad, pathEntitiesFlush:
			fmt.Fprint(w, `{"code":0,"message":"success"}`)
		case pathEntitiesInsert:
			n := 0
			if data, ok := req["data"].([]any); ok {
				n = len(data)
			}
			fmt.Fprintf(w, `{"code":0,"data":{"insertCount":%d,"insertIds":[]}}`, n)
		case pathEntitiesSearch:
			fmt.Fprint(w, `{"code":0,"data":[
				{"pk":1,"distance":0.05,"text":"hello","meta":{"src":"wiki"}},
				{"pk":2,"distance":0.08,"text":"world"}
			]}`)
		default:
			fmt.Fprintf(w, `{"code":100,"message":"unknown path %s"}`, path)
		}
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// store 基于模拟服务构造 Store.
// [EN] Build a Store against the mock server.
func (m *mockServer) store(opts ...Option) *Store {
	return newWithPoster(
		transport.NewClient(transport.WithHTTPClient(m.srv.Client())),
		m.srv.URL+apiVersion, opts...)
}

// bodyOf 取指定路径第 idx 次请求体.
// [EN] Get the idx-th request body for a path.
func (m *mockServer) bodyOf(path string, idx int) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	reqs := m.requests[path]
	if len(reqs) == 0 {
		return nil
	}
	if idx >= len(reqs) {
		idx = len(reqs) - 1
	}
	return reqs[idx]
}

// callsOf 路径调用次数.
// [EN] Call count of a path.
func (m *mockServer) callsOf(path string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests[path])
}

// TestInit_Create 验证全新建链路（has → create → load）与 schema 形态.
// [EN] Verify creation (has → create → load) and the schema.
func TestInit_Create(t *testing.T) {
	m := newMockServer(t, false, "")
	s := m.store(WithDimensions(8))

	require.NoError(t, s.Init(context.Background()))
	assert.Equal(t, 1, m.callsOf(pathCollectionsHas))
	assert.Equal(t, 1, m.callsOf(pathCollectionsCreate))
	assert.Equal(t, 1, m.callsOf(pathCollectionsLoad))

	create := m.bodyOf(pathCollectionsCreate, 0)
	assert.Equal(t, "LangChainGoCollection", create["collectionName"])
	schema := create["schema"].(map[string]any)
	assert.Equal(t, true, schema["autoId"])
	fields := schema["fields"].([]any)
	require.Len(t, fields, 4)
	assert.Equal(t, "pk", fields[0].(map[string]any)["fieldName"])
	assert.Equal(t, true, fields[0].(map[string]any)["isPrimary"])
	vec := fields[3].(map[string]any)
	assert.Equal(t, "vector", vec["fieldName"])
	assert.Equal(t, "8", vec["elementTypeParams"].(map[string]any)["dim"])
	idx := create["indexParams"].([]any)[0].(map[string]any)
	assert.Equal(t, "vector", idx["fieldName"])
	assert.Equal(t, "COSINE", idx["metricType"])
	assert.Equal(t, 8, s.dim)
}

// TestInit_CreateMissingDim 验证建集合缺维度报错.
// [EN] Verify the missing-dimension error on creation.
func TestInit_CreateMissingDim(t *testing.T) {
	m := newMockServer(t, false, "")
	s := m.store()
	err := s.Init(context.Background())
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
	assert.Equal(t, 0, m.callsOf(pathCollectionsCreate))
}

// TestInit_Existing 验证存量链路（has → describe 对齐维度，已加载免 load）.
// [EN] Verify the existing path (has → describe; loaded skips load).
func TestInit_Existing(t *testing.T) {
	m := newMockServer(t, true, "Loaded")
	s := m.store()

	require.NoError(t, s.Init(context.Background()))
	assert.Equal(t, 1, m.callsOf(pathCollectionsDescr))
	assert.Equal(t, 0, m.callsOf(pathCollectionsCreate))
	assert.Equal(t, 0, m.callsOf(pathCollectionsLoad))
	assert.Equal(t, 2, s.dim)
}

// TestInit_ExistingNotLoaded 验证未加载存量集合触发 load.
// [EN] Verify load is triggered for an unloaded existing collection.
func TestInit_ExistingNotLoaded(t *testing.T) {
	m := newMockServer(t, true, "NotLoad")
	s := m.store()

	require.NoError(t, s.Init(context.Background()))
	assert.Equal(t, 1, m.callsOf(pathCollectionsLoad))
}

// TestAddDocuments 验证写入载荷与刷盘.
// [EN] Verify insert payloads and flush.
func TestAddDocuments(t *testing.T) {
	m := newMockServer(t, true, "Loaded")
	s := m.store(WithDimensions(2))
	require.NoError(t, s.Init(context.Background()))

	docs := []llmx.Document{
		{PageContent: "hello", Metadata: map[string]any{"src": "wiki"}},
		{PageContent: "world"},
	}
	require.NoError(t, s.AddDocuments(context.Background(), docs, [][]float64{{1, 0}, {0, 1}}))

	ins := m.bodyOf(pathEntitiesInsert, 0)
	assert.Equal(t, "LangChainGoCollection", ins["collectionName"])
	rows := ins["data"].([]any)
	require.Len(t, rows, 2)
	row0 := rows[0].(map[string]any)
	assert.Equal(t, "hello", row0["text"])
	assert.Equal(t, []any{float64(1), float64(0)}, row0["vector"])
	assert.Equal(t, map[string]any{"src": "wiki"}, row0["meta"])
	assert.NotContains(t, rows[1].(map[string]any), "meta")
	assert.Equal(t, 1, m.callsOf(pathEntitiesFlush))
}

// TestAddDocuments_SkipFlush 验证跳过刷盘.
// [EN] Verify flush skipping.
func TestAddDocuments_SkipFlush(t *testing.T) {
	m := newMockServer(t, true, "Loaded")
	s := m.store(WithDimensions(2), WithSkipFlush())
	require.NoError(t, s.Init(context.Background()))

	require.NoError(t, s.AddDocuments(context.Background(),
		[]llmx.Document{{PageContent: "a"}}, [][]float64{{1, 0}}))
	assert.Equal(t, 0, m.callsOf(pathEntitiesFlush))
}

// TestAddDocuments_Validation 验证数量与维度校验.
// [EN] Verify count and dimension validation.
func TestAddDocuments_Validation(t *testing.T) {
	s := newMockServer(t, true, "Loaded").store(WithDimensions(3))

	err := s.AddDocuments(context.Background(),
		[]llmx.Document{{PageContent: "a"}},
		[][]float64{{1, 0}, {0, 1}})
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)

	err = s.AddDocuments(context.Background(),
		[]llmx.Document{{PageContent: "a"}},
		[][]float64{{1, 0}})
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)

	_, err = s.SimilaritySearch(context.Background(), []float64{1, 0}, 5)
	assert.ErrorIs(t, err, llmx.ErrInvalidVectors)
}

// TestSimilaritySearch 验证检索载荷与行集还原.
// [EN] Verify search payloads and row restoration.
func TestSimilaritySearch(t *testing.T) {
	m := newMockServer(t, true, "Loaded")
	s := m.store(WithDimensions(2))
	require.NoError(t, s.Init(context.Background()))

	docs, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 5, llmx.Filter{"src": "wiki"})
	require.NoError(t, err)
	require.Len(t, docs, 2)
	assert.Equal(t, "hello", docs[0].PageContent)
	assert.Equal(t, "wiki", docs[0].Metadata["src"])
	assert.Equal(t, "world", docs[1].PageContent)
	assert.Nil(t, docs[1].Metadata)

	search := m.bodyOf(pathEntitiesSearch, 0)
	assert.Equal(t, []any{[]any{float64(1), float64(0)}}, search["data"])
	assert.Equal(t, "vector", search["annsField"])
	assert.Equal(t, float64(5), search["limit"])
	assert.Equal(t, []any{"text", "meta"}, search["outputFields"])
	assert.Equal(t, `meta["src"] == "wiki"`, search["filter"])
	assert.Equal(t, map[string]any{"metricType": "COSINE"}, search["searchParams"])
}

// TestBuildFilterExpr 验证过滤表达式（排序/类型/转义）.
// [EN] Verify filter expressions (ordering/types/escaping).
func TestBuildFilterExpr(t *testing.T) {
	assert.Equal(t, "", buildFilterExpr("meta", nil))
	assert.Equal(t, `meta["a"] == "x"`,
		buildFilterExpr("meta", []llmx.Filter{{"a": "x"}}))
	assert.Equal(t, `meta["a"] == 1 && meta["b"] == "y"`,
		buildFilterExpr("meta", []llmx.Filter{{"b": "y", "a": 1}}))
	assert.Equal(t, `meta["k"] == "he said \"hi\""`,
		buildFilterExpr("meta", []llmx.Filter{{"k": `he said "hi"`}}))
	assert.Equal(t, `meta["ok"] == true`,
		buildFilterExpr("meta", []llmx.Filter{{"ok": true}}))
}

// TestVectorDimFromDescribe 验证维度提取与缺失报错.
// [EN] Verify dimension extraction and the missing-dimension error.
func TestVectorDimFromDescribe(t *testing.T) {
	d := &wireDescribeData{Fields: []*wireField{
		{FieldName: "vector", DataType: dataTypeFloatVector, ElementTypeParams: map[string]string{"dim": "1536"}},
	}}
	dim, err := vectorDimFromDescribe(d, "vector")
	require.NoError(t, err)
	assert.Equal(t, 1536, dim)

	_, err = vectorDimFromDescribe(&wireDescribeData{Fields: []*wireField{}}, "vector")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// TestRowsToDocuments 验证行集还原（对象/字符串元数据）.
// [EN] Verify row restoration (object/string metadata).
func TestRowsToDocuments(t *testing.T) {
	rows := []map[string]any{
		{"text": "a", "meta": map[string]any{"k": "v"}},
		{"text": "b", "meta": `{"n":1}`},
		{"text": "c"},
	}
	docs := rowsToDocuments(rows, "text", "meta")
	require.Len(t, docs, 3)
	assert.Equal(t, map[string]any{"k": "v"}, docs[0].Metadata)
	assert.Equal(t, float64(1), docs[1].Metadata["n"])
	assert.Nil(t, docs[2].Metadata)
}
