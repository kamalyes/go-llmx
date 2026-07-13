/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-13 21:07:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-13 21:26:41
 * @FilePath: \go-llmx\vectorstores\pgvector\pgvector_test.go
 * @Description: pgvector 单测 —— fake 数据库通道注入：编码/DDL 构造/检索 SQL
 * 参数化/批次写入/行集还原全路径覆盖，零 PostgreSQL 依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcpgvector

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	llmx "github.com/kamalyes/go-llmx"
)

// fakeDB 记录式数据库通道.
// [EN] Recording database channel.
type fakeDB struct {
	// execs DDL/写语句序列.
	// [EN] DDL/write statement sequence.
	execs []statement

	// queries 查询语句序列.
	// [EN] Query statement sequence.
	queries []statement

	// rows 查询返回行集（轮询消费）.
	// [EN] Query result rows (consumed in order).
	rows [][][]any
}

// Exec 实现 db.
// [EN] Implement db.
func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) error {
	f.execs = append(f.execs, statement{sql: sql, args: args})
	return nil
}

// Query 实现 db.
// [EN] Implement db.
func (f *fakeDB) Query(_ context.Context, sql string, args ...any) ([][]any, error) {
	f.queries = append(f.queries, statement{sql: sql, args: args})
	if len(f.rows) == 0 {
		return nil, nil
	}
	rows := f.rows[0]
	f.rows = f.rows[1:]
	return rows, nil
}

// SendBatch 实现 db.
// [EN] Implement db.
func (f *fakeDB) SendBatch(_ context.Context, stmts []statement) error {
	f.execs = append(f.execs, stmts...)
	return nil
}

// TestEncodeVectorText 验证 pgvector 文本编码（float32 精度）.
// [EN] Verify pgvector text encoding (float32 precision).
func TestEncodeVectorText(t *testing.T) {
	assert.Equal(t, "[0.5,-1.25]", encodeVectorText([]float64{0.5, -1.25}))
	assert.Equal(t, "[0.1]", encodeVectorText([]float64{0.1}))
	assert.Equal(t, "[]", encodeVectorText(nil))
}

// TestOperatorFor 验证距离算子映射.
// [EN] Verify operator mapping.
func TestOperatorFor(t *testing.T) {
	assert.Equal(t, "<=>", operatorFor(DistanceCosine))
	assert.Equal(t, "<->", operatorFor(DistanceL2))
	assert.Equal(t, "<#>", operatorFor(DistanceIP))
	assert.Equal(t, "<=>", operatorFor("anything"))
}

// TestBuildDDL 验证 DDL 序列（表结构/定型列/HNSW WITH 参数）.
// [EN] Verify the DDL sequence (tables/typed column/HNSW WITH).
func TestBuildDDL(t *testing.T) {
	s := newWithDB(&fakeDB{}, WithVectorDimensions(8), WithHNSWIndex(16, 200, DistanceCosine))
	stmts := s.buildDDL()
	require.Len(t, stmts, 5)

	assert.Equal(t, "CREATE EXTENSION IF NOT EXISTS vector", stmts[0].sql)
	assert.Contains(t, stmts[1].sql, "CREATE TABLE IF NOT EXISTS langchain_pg_collection")
	assert.Contains(t, stmts[2].sql, "CREATE TABLE IF NOT EXISTS langchain_pg_embedding")
	assert.Contains(t, stmts[2].sql, "embedding vector(8)")
	assert.Contains(t, stmts[2].sql, "FOREIGN KEY (collection_id) REFERENCES langchain_pg_collection (uuid) ON DELETE CASCADE")
	assert.Contains(t, stmts[4].sql, "USING hnsw (embedding vector_cosine_ops)")
	assert.Contains(t, stmts[4].sql, "WITH (m=16, ef_construction=200)")

	untyped := newWithDB(&fakeDB{}).buildDDL()
	assert.Contains(t, untyped[2].sql, "embedding vector,\n")
	assert.Len(t, untyped, 4)
}

// TestInit 验证初始化流程（DDL + upsert + UUID 获取）.
// [EN] Verify initialization (DDL + upsert + UUID acquisition).
func TestInit(t *testing.T) {
	f := &fakeDB{rows: [][][]any{{{"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d"}}}}
	s := newWithDB(f)

	require.NoError(t, s.Init(context.Background()))
	require.Len(t, f.execs, 5) // ext + 2 tables + collection_id index + upsert
	require.Len(t, f.queries, 1)

	assert.Contains(t, f.execs[4].sql, "INSERT INTO langchain_pg_collection (uuid, name, cmetadata)")
	assert.Equal(t, "langchain", f.execs[4].args[1])
	assert.Contains(t, f.queries[0].sql, "SELECT uuid FROM langchain_pg_collection WHERE name = $1")
	assert.Equal(t, "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d", s.collectionUUID)
}

// TestAddDocuments 验证批量写入形态（UUIDv4/向量文本/cmetadata JSON/外键）.
// [EN] Verify batch writes (UUIDv4/vector text/cmetadata JSON/FK).
func TestAddDocuments(t *testing.T) {
	f := &fakeDB{rows: [][][]any{{{"c1"}}}}
	s := newWithDB(f)
	require.NoError(t, s.Init(context.Background()))
	nExecs := len(f.execs)

	docs := []llmx.Document{
		{PageContent: "hello", Metadata: map[string]any{"src": "wiki"}},
		{PageContent: "world"},
	}
	require.NoError(t, s.AddDocuments(context.Background(), docs, [][]float64{{1, 0}, {0, 1}}))

	inserts := f.execs[nExecs:]
	require.Len(t, inserts, 2)
	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for i, ins := range inserts {
		assert.Contains(t, ins.sql, "INSERT INTO langchain_pg_embedding")
		assert.Regexp(t, uuidRe, ins.args[0])
		assert.Equal(t, docs[i].PageContent, ins.args[1])
		assert.Equal(t, "c1", ins.args[4])
	}
	assert.Equal(t, "[1,0]", inserts[0].args[2])
	assert.Equal(t, `{"src":"wiki"}`, inserts[0].args[3])
	assert.Nil(t, inserts[1].args[3])
}

// TestAddDocuments_Validation 验证数量与维度校验（定型列）.
// [EN] Verify count and dimension validation (typed column).
func TestAddDocuments_Validation(t *testing.T) {
	s := newWithDB(&fakeDB{}, WithVectorDimensions(3))

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

// TestBuildSearch 验证检索 SQL（定型免维度子句 / 过滤参数化 / 算子自适应）.
// [EN] Verify search SQL (no dims clause when typed / parameterized filters / operator).
func TestBuildSearch(t *testing.T) {
	// 未定型：含 vector_dims 过滤，参数序 name/dims/filters/vector/limit
	s := newWithDB(&fakeDB{})
	sql, args := s.buildSearch([]float64{1, 0}, 5, []llmx.Filter{{"b": "y", "a": "x"}})
	assert.Contains(t, sql, "WHERE c.name = $1 AND vector_dims(e.embedding) = $2")
	assert.Contains(t, sql, "(e.cmetadata ->> $3) = $4")
	assert.Contains(t, sql, "(e.cmetadata ->> $5) = $6")
	assert.Contains(t, sql, "ORDER BY e.embedding <=> $7::vector LIMIT $8")
	assert.Equal(t, []any{"langchain", 2, "a", "x", "b", "y", "[1,0]", 5}, args)

	// 定型：免维度子句；L2 索引换算子
	typed := newWithDB(&fakeDB{}, WithVectorDimensions(2), WithHNSWIndex(16, 200, DistanceL2))
	sql, args = typed.buildSearch([]float64{1, 0}, 0, nil)
	assert.NotContains(t, sql, "vector_dims")
	assert.Contains(t, sql, "ORDER BY e.embedding <-> $2::vector LIMIT $3")
	assert.Equal(t, []any{"langchain", "[1,0]", 10}, args)
}

// TestSimilaritySearch 验证检索全链路（SQL 执行 + 行集还原）.
// [EN] Verify the full search path (SQL execution + row restoration).
func TestSimilaritySearch(t *testing.T) {
	f := &fakeDB{rows: [][][]any{{
		{"hello", []byte(`{"src":"wiki"}`)},
		{"world", nil},
	}}}
	s := newWithDB(f, WithVectorDimensions(2))

	docs, err := s.SimilaritySearch(context.Background(), []float64{1, 0}, 5, llmx.Filter{"src": "wiki"})
	require.NoError(t, err)
	require.Len(t, docs, 2)
	assert.Equal(t, "hello", docs[0].PageContent)
	assert.Equal(t, "wiki", docs[0].Metadata["src"])
	assert.Equal(t, "world", docs[1].PageContent)
	assert.Nil(t, docs[1].Metadata)
	require.Len(t, f.queries, 1)
	assert.Contains(t, f.queries[0].sql, "ORDER BY e.embedding <=> $4::vector")
}

// TestRowsToDocuments 验证 cmetadata 兼容形态（string/[]byte/nil/坏 JSON）.
// [EN] Verify cmetadata forms (string/[]byte/nil/bad JSON).
func TestRowsToDocuments(t *testing.T) {
	docs, err := rowsToDocuments([][]any{
		{"a", `{"k":"v"}`},
		{"b", []byte(`{"n":1}`)},
		{"c", nil},
	})
	require.NoError(t, err)
	require.Len(t, docs, 3)
	assert.Equal(t, map[string]any{"k": "v"}, docs[0].Metadata)
	assert.Equal(t, float64(1), docs[1].Metadata["n"])
	assert.Nil(t, docs[2].Metadata)

	_, err = rowsToDocuments([][]any{{"a", "{bad"}})
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// TestUUID 验证 UUID 编解码（生成格式/字节还原）.
// [EN] Verify UUID codec (generated format/byte restoration).
func TestUUID(t *testing.T) {
	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for i := 0; i < 8; i++ {
		assert.Regexp(t, uuidRe, newUUID())
	}
	assert.Equal(t, "00010203-0405-0607-0809-0a0b0c0d0e0f",
		uuidString([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))
	assert.Equal(t, "plain", uuidString("plain"))
}
