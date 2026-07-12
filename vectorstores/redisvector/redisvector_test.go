/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-12 22:11:09
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-12 22:16:33
 * @FilePath: \go-llmx\vectorstores\redisvector\redisvector_test.go
 * @Description: redisvector 单测 —— fake 命令通道注入：编码/KNN 构造/
 * 响应解析/主键语义/RETURN 裁剪全路径覆盖，零 Redis 依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcredisvector

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	llmx "github.com/kamalyes/go-llmx"
)

// fakeRunner 记录式命令通道.
// [EN] Recording command channel.
type fakeRunner struct {
	// calls 单命令序列.
	// [EN] Single-command sequence.
	calls [][]any

	// batches pipeline 批次.
	// [EN] Pipeline batches.
	batches [][][]any

	// result 单命令返回值.
	// [EN] Single-command return value.
	result any

	// err 预置错误.
	// [EN] Preset error.
	err error
}

// Do 实现 commandRunner.
// [EN] Implement commandRunner.
func (f *fakeRunner) Do(_ context.Context, args ...any) (any, error) {
	f.calls = append(f.calls, args)
	return f.result, f.err
}

// PipelineDo 实现 commandRunner.
// [EN] Implement commandRunner.
func (f *fakeRunner) PipelineDo(_ context.Context, batches [][]any) error {
	f.batches = append(f.batches, batches)
	return nil
}

// joinArgs 参数序列字符串化（断言辅助，二进制字段跳过）.
// [EN] Stringify args (assertion helper; binary fields skipped).
func joinArgs(args []any) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		switch v := a.(type) {
		case string:
			parts = append(parts, v)
		case int:
			parts = append(parts, strconv.Itoa(v))
		default:
			parts = append(parts, "?")
		}
	}
	return strings.Join(parts, " ")
}

// TestEncodeVector 验证 FLOAT32 小端编码.
// [EN] Verify FLOAT32 little-endian encoding.
func TestEncodeVector(t *testing.T) {
	b := encodeVector([]float64{1.5, -2.25})
	require.Len(t, b, 8)
	assert.Equal(t, float32(1.5), math.Float32frombits(binary.LittleEndian.Uint32(b[0:4])))
	assert.Equal(t, float32(-2.25), math.Float32frombits(binary.LittleEndian.Uint32(b[4:8])))
}

// TestBuildCreateArgs 验证 FT.CREATE 参数（HNSW + TAG 过滤字段）.
// [EN] Verify FT.CREATE args (HNSW + TAG filter fields).
func TestBuildCreateArgs(t *testing.T) {
	args := buildCreateArgs("idx", "doc:idx", 8, []string{"src"})
	joined := joinArgs(args)
	assert.Contains(t, joined, "FT.CREATE idx ON HASH PREFIX 1 doc:idx: SCHEMA content TEXT")
	assert.Contains(t, joined, "content_vector VECTOR HNSW 6 TYPE FLOAT32 DIM 8 DISTANCE_METRIC COSINE")
	assert.Contains(t, joined, "src TAG")
}

// TestBuildKNNQuery 验证 KNN 查询串（无过滤 / TAG 过滤 / topK 兜底）.
// [EN] Verify the KNN query string (bare / TAG filtered / topK fallback).
func TestBuildKNNQuery(t *testing.T) {
	assert.Equal(t, "*=>[KNN 5 @content_vector $vec AS distance]", buildKNNQuery(5, nil))
	assert.Equal(t,
		"(@src:{wiki})=>[KNN 5 @content_vector $vec AS distance]",
		buildKNNQuery(5, []llmx.Filter{{"src": "wiki"}}))
	q := buildKNNQuery(5, []llmx.Filter{{"a": "x", "b": 2}})
	assert.Contains(t, q, "(@a:{x} @b:{2})")
	assert.Contains(t, buildKNNQuery(0, nil), "KNN 10 ")
}

// TestBuildSearchArgs 验证 FT.SEARCH 参数（RETURN 裁剪 + blob 注入）.
// [EN] Verify FT.SEARCH args (RETURN trimming + blob injection).
func TestBuildSearchArgs(t *testing.T) {
	vec := []float64{0.1, 0.2}
	args := buildSearchArgs("idx", "*", vec, 3, []string{"src"})
	joined := joinArgs(args)
	assert.Contains(t, joined, "FT.SEARCH idx *")
	assert.Contains(t, joined, "RETURN 2 content src")
	assert.Contains(t, joined, "SORTBY distance ASC")
	assert.Contains(t, joined, "PARAMS 2 vec")
	var sawBlob bool
	for _, a := range args {
		if b, ok := a.([]byte); ok {
			sawBlob = true
			assert.Equal(t, encodeVector(vec), b)
		}
	}
	assert.True(t, sawBlob, "vector blob must be injected as binary")
}

// TestParseSearchResponse 验证嵌套数组解析（content + 元数据 + id）.
// [EN] Verify nested-array parsing (content + metadata + id).
func TestParseSearchResponse(t *testing.T) {
	v := []any{
		int64(2),
		[]any{"doc:idx:1", []any{"content", "hello", "src", "wiki"}},
		[]any{"doc:idx:2", []any{"content", "world"}},
	}
	docs := parseSearchResponse(v)
	require.Len(t, docs, 2)
	assert.Equal(t, "hello", docs[0].PageContent)
	assert.Equal(t, "wiki", docs[0].Metadata["src"])
	assert.Equal(t, "doc:idx:1", docs[0].Metadata["id"])
	assert.Equal(t, "world", docs[1].PageContent)
	assert.Nil(t, parseSearchResponse("bad"))
}

// TestAddDocuments_KeySemantics 验证主键语义（ids/keys 优先，否则随机 hex）.
// [EN] Verify key semantics (ids/keys first, else random hex).
func TestAddDocuments_KeySemantics(t *testing.T) {
	f := &fakeRunner{}
	s := newWithRunner(f, WithIndex("idx"), WithDimensions(2))

	docs := []llmx.Document{
		{PageContent: "a", Metadata: map[string]any{"ids": "42"}},
		{PageContent: "b", Metadata: map[string]any{"keys": "k9"}},
		{PageContent: "c"},
	}
	require.NoError(t, s.AddDocuments(context.Background(), docs, [][]float64{{1, 0}, {0, 1}, {1, 1}}))
	require.Len(t, f.batches, 1)
	batch := f.batches[0]

	assert.Equal(t, "doc:idx:42", batch[0][1])
	assert.Equal(t, "doc:idx:k9", batch[1][1])
	key, _ := batch[2][1].(string)
	assert.True(t, strings.HasPrefix(key, "doc:idx:"), "random key prefix")
	assert.NotEqual(t, "doc:idx:42", key)

	// HSET 字段形态：content + content_vector + 元数据平铺
	assert.Equal(t, "HSET", batch[0][0])
	assert.Equal(t, "content", batch[0][2])
	assert.Equal(t, "a", batch[0][3])
	assert.Equal(t, "content_vector", batch[0][4])
	_, isBlob := batch[0][5].([]byte)
	assert.True(t, isBlob)
	assert.Equal(t, "ids", batch[0][6])
	assert.Equal(t, "42", batch[0][7])
}

// TestAddDocuments_Validation 验证维度与数量校验.
// [EN] Verify dimension and count validation.
func TestAddDocuments_Validation(t *testing.T) {
	s := newWithRunner(&fakeRunner{}, WithDimensions(3))

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

// TestSimilaritySearch_KnownFields 验证写入字段并入 RETURN（裁剪生效）.
// [EN] Verify written fields merged into RETURN (trimming works).
func TestSimilaritySearch_KnownFields(t *testing.T) {
	f := &fakeRunner{}
	s := newWithRunner(f, WithIndex("idx"), WithDimensions(2))

	require.NoError(t, s.AddDocuments(context.Background(),
		[]llmx.Document{{PageContent: "a", Metadata: map[string]any{"src": "wiki", "topic": "go"}}},
		[][]float64{{1, 0}}))

	_, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 5)
	require.NoError(t, err)
	require.Len(t, f.calls, 1)
	joined := joinArgs(f.calls[0])
	assert.Contains(t, joined, "content src topic")
}

// TestSimilaritySearch_Filters 验证过滤条件进入 KNN 前缀.
// [EN] Verify filters flow into the KNN prefix.
func TestSimilaritySearch_Filters(t *testing.T) {
	f := &fakeRunner{}
	s := newWithRunner(f, WithIndex("idx"), WithDimensions(2))

	_, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 5,
		llmx.Filter{"src": "wiki"})
	require.NoError(t, err)
	require.Len(t, f.calls, 1)
	joined := joinArgs(f.calls[0])
	assert.Contains(t, joined, "(@src:{wiki})")
}

// TestInitIndex 验证建索引与 already exists 幂等.
// [EN] Verify index creation and already-exists idempotency.
func TestInitIndex(t *testing.T) {
	ok := &fakeRunner{}
	s := newWithRunner(ok, WithIndex("idx"), WithFilterFields("src"))
	assert.NoError(t, s.InitIndex(context.Background()))
	require.Len(t, ok.calls, 1)
	assert.Contains(t, joinArgs(ok.calls[0]), "FT.CREATE idx")

	dup := &fakeRunner{err: errors.New("Index already exists")}
	assert.NoError(t, newWithRunner(dup).InitIndex(context.Background()))

	fail := &fakeRunner{err: errors.New("connection refused")}
	assert.Error(t, newWithRunner(fail).InitIndex(context.Background()))
}
