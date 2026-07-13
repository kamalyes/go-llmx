/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-13 21:07:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-13 21:26:41
 * @FilePath: \go-llmx\vectorstores\pgvector\wire.go
 * @Description: pgvector 协议层 —— 向量文本编码（免 pgvector-go 依赖）、
 * 建表/检索 SQL 构造（过滤参数化防注入）、行集到文档还原、UUID 编解码
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcpgvector

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// encodeVectorText 编码向量为 pgvector 文本形态 "[0.1,0.2]"（float32 精度，
// 与 langchaingo 的 []float32 编码位级一致）.
// [EN] Encode a vector as pgvector text "[0.1,0.2]" (float32 precision,
// bit-compatible with langchaingo's []float32 encoding).
func encodeVectorText(v []float64) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(float32(x)), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// operatorFor 距离算子类 → 检索算子.
// [EN] Operator class to search operator.
func operatorFor(distanceFunction string) string {
	switch distanceFunction {
	case DistanceL2:
		return "<->"
	case DistanceIP:
		return "<#>"
	default:
		return "<=>"
	}
}

// buildDDL 构造建表语句序列（扩展 → 集合表 → 嵌入表 → HNSW 索引）.
// [EN] Build the DDL sequence (extension → collection table → embedding table → HNSW).
//
// 表结构与 langchaingo/pgvector 完全同构（列名/外键/级联删除），
// typedVector 为空时列为无维度约束的 vector
func (s *Store) buildDDL() []statement {
	tables := []statement{
		{sql: "CREATE EXTENSION IF NOT EXISTS vector"},
		{sql: fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	name varchar,
	cmetadata json,
	"uuid" uuid NOT NULL,
	UNIQUE (name),
	PRIMARY KEY (uuid))`, s.collectionTable)},
	}
	embeddingDDL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	collection_id uuid,
	embedding vector%s,
	document varchar,
	cmetadata json,
	"uuid" uuid NOT NULL,
	CONSTRAINT %s_collection_id_fkey
		FOREIGN KEY (collection_id) REFERENCES %s (uuid) ON DELETE CASCADE,
	PRIMARY KEY (uuid))`, s.embeddingTable, s.typedVectorClause(), s.embeddingTable, s.collectionTable)
	tables = append(tables,
		statement{sql: embeddingDDL},
		statement{sql: fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_collection_id ON %s (collection_id)",
			s.embeddingTable, s.embeddingTable)},
	)
	if s.hnsw != nil {
		idx := fmt.Sprintf(
			"CREATE INDEX IF NOT EXISTS %s_embedding_hnsw ON %s USING hnsw (embedding %s)",
			s.embeddingTable, s.embeddingTable, s.hnsw.distanceFunction)
		if s.hnsw.m > 0 && s.hnsw.efConstruction > 0 {
			idx += fmt.Sprintf(" WITH (m=%d, ef_construction=%d)", s.hnsw.m, s.hnsw.efConstruction)
		}
		tables = append(tables, statement{sql: idx})
	}
	return tables
}

// typedVectorClause 定型向量列子句（维度已知时 vector(N)，空串为不定型）.
// [EN] Typed vector clause (vector(N) when known; empty when untyped).
func (s *Store) typedVectorClause() string {
	if s.vectorDimensions > 0 {
		return fmt.Sprintf("(%d)", s.vectorDimensions)
	}
	return ""
}

// buildSearch 构造相似度检索 SQL 与参数（参数化过滤，防注入）.
// [EN] Build the similarity-search SQL and args (parameterized filters).
//
// 参数序：$1 集合名 → [未定型维度 $2] → 过滤键值对 → 向量 → LIMIT；
// langchaingo 将过滤值内联进 SQL（注入风险），此处全部参数化
func (s *Store) buildSearch(query []float64, topK int, filters []llmx.Filter) (string, []any) {
	if topK <= 0 {
		topK = DefaultTopK
	}
	args := make([]any, 0, 6+len(filters)*2)
	args = append(args, s.collectionName)
	n := 1

	where := make([]string, 0, 2+len(filters))
	if s.vectorDimensions <= 0 {
		n++
		where = append(where, fmt.Sprintf("vector_dims(e.embedding) = $%d", n))
		args = append(args, len(query))
	}

	// 键排序保证 SQL 形态确定（map 迭代序随机）
	keys := make([]string, 0, len(filters))
	for _, f := range filters {
		for k := range f {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		n++
		kp := n
		n++
		vp := n
		where = append(where, fmt.Sprintf("(e.cmetadata ->> $%d) = $%d", kp, vp))
		var val any
		for _, f := range filters {
			if v, ok := f[k]; ok {
				val = v
			}
		}
		args = append(args, k, val)
	}

	n++
	vecParam := n
	n++
	limitParam := n

	sql := fmt.Sprintf(`SELECT e.document, e.cmetadata FROM %s e
	JOIN %s c ON e.collection_id = c.uuid
	WHERE c.name = $1`, s.embeddingTable, s.collectionTable)
	if len(where) > 0 {
		sql += " AND " + strings.Join(where, " AND ")
	}
	sql += fmt.Sprintf(" ORDER BY e.embedding %s $%d::vector LIMIT $%d",
		operatorFor(s.searchOperator()), vecParam, limitParam)
	args = append(args, encodeVectorText(query), topK)
	return sql, args
}

// searchOperator 检索算子依据（未配 HNSW 时默认余弦）.
// [EN] Operator basis (cosine when HNSW is unset).
func (s *Store) searchOperator() string {
	if s.hnsw != nil {
		return s.hnsw.distanceFunction
	}
	return DistanceCosine
}

// rowsToDocuments 检索行集还原为文档（cmetadata 兼容 string/[]byte/nil）.
// [EN] Restore result rows to documents (cmetadata as string/[]byte/nil).
func rowsToDocuments(rows [][]any) ([]llmx.Document, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	docs := make([]llmx.Document, 0, len(rows))
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		doc := llmx.Document{PageContent: asString(row[0])}
		switch meta := row[1].(type) {
		case nil:
		case string:
			if meta != "" && meta != "null" {
				if err := json.Unmarshal([]byte(meta), &doc.Metadata); err != nil {
					return nil, fmt.Errorf("%w: cmetadata: %v", llmx.ErrInvalidRequest, err)
				}
			}
		case []byte:
			if len(meta) > 0 {
				if err := json.Unmarshal(meta, &doc.Metadata); err != nil {
					return nil, fmt.Errorf("%w: cmetadata: %v", llmx.ErrInvalidRequest, err)
				}
			}
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

// asString 宽松字符串还原.
// [EN] Lenient string restoration.
func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

// newUUID 生成 UUIDv4 文本（crypto/rand，免第三方依赖）.
// [EN] Generate a UUIDv4 string (crypto/rand; no third-party deps).
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst)
}

// uuidString 数据库返回值还原为 UUID 文本（pgx 解码为 [16]byte）.
// [EN] Restore a database value to UUID text (pgx decodes as [16]byte).
func uuidString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case [16]byte:
		b := x
		return newUUIDFrom(b[:])
	case []byte:
		if len(x) == 16 {
			return newUUIDFrom(x)
		}
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}

// newUUIDFrom 字节还原 UUID 文本.
// [EN] Bytes to UUID text.
func newUUIDFrom(b []byte) string {
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst)
}
