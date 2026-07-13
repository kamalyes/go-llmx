/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-13 21:19:38
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-13 21:26:41
 * @FilePath: \go-llmx\vectorstores\pgvector\store.go
 * @Description: PostgreSQL 向量库（pgvector 扩展）—— pgx v5 连接池直连.
 * 表结构与 langchaingo/pgvector 完全同构（langchain_pg_embedding 双表 + 集合
 * 语义 + 外键级联），存量库平滑读写；过滤参数化防注入，批量写入走
 * pgx.Batch 单往返，检索算子随 HNSW 距离函数自适应
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcpgvector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	llmx "github.com/kamalyes/go-llmx"
)

// statement 单条 SQL 及其参数.
// [EN] One SQL statement with its arguments.
type statement struct {
	sql  string
	args []any
}

// db 窄数据库接口（pgx 适配器满足；测试注入零依赖）.
// [EN] Narrow database interface (satisfied by the pgx adapter).
type db interface {
	// Exec 执行单条语句.
	// [EN] Execute one statement.
	Exec(ctx context.Context, sql string, args ...any) error

	// Query 查询并物化全部行（每行为列值切片）.
	// [EN] Query and materialize all rows.
	Query(ctx context.Context, sql string, args ...any) ([][]any, error)

	// SendBatch 单往返执行语句批次.
	// [EN] Execute a statement batch in one round-trip.
	SendBatch(ctx context.Context, stmts []statement) error
}

// pgxConn 统一 *pgx.Conn 与 *pgxpool.Pool 的执行面.
// [EN] Unify the exec surface of *pgx.Conn and *pgxpool.Pool.
type pgxConn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults
}

// pgxAdapter 将 pgx 连接适配为窄接口.
// [EN] Adapt a pgx connection to the narrow interface.
type pgxAdapter struct {
	c pgxConn
}

// Exec 实现 db.
// [EN] Implement db.
func (a *pgxAdapter) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := a.c.Exec(ctx, sql, args...)
	return err
}

// Query 实现 db（物化行集）.
// [EN] Implement db (materialized rows).
func (a *pgxAdapter) Query(ctx context.Context, sql string, args ...any) ([][]any, error) {
	rows, err := a.c.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		out = append(out, vals)
	}
	return out, rows.Err()
}

// SendBatch 实现 db（pgx.Batch 单往返）.
// [EN] Implement db (pgx.Batch in one round-trip).
func (a *pgxAdapter) SendBatch(ctx context.Context, stmts []statement) error {
	b := &pgx.Batch{}
	for _, s := range stmts {
		b.Queue(s.sql, s.args...)
	}
	return a.c.SendBatch(ctx, b).Close()
}

// hnswIndex HNSW 索引配置.
// [EN] HNSW index configuration.
type hnswIndex struct {
	// m 每层最大连接数（默认 16）.
	// [EN] Max connections per layer (default 16).
	m int

	// efConstruction 构建期候选列表大小（默认 64）.
	// [EN] Candidate list size during construction (default 64).
	efConstruction int

	// distanceFunction 距离算子类（vector_cosine_ops 等）.
	// [EN] Distance operator class.
	distanceFunction string
}

// Store PostgreSQL 向量库.
// [EN] PostgreSQL vector store.
type Store struct {
	// db 数据库通道.
	// [EN] Database channel.
	db db

	// collectionName 集合名（多租户隔离语义，langchaingo 同构）.
	// [EN] Collection name (tenant isolation, langchaingo-compatible).
	collectionName string

	// embeddingTable 嵌入表名.
	// [EN] Embedding table name.
	embeddingTable string

	// collectionTable 集合表名.
	// [EN] Collection table name.
	collectionTable string

	// collectionUUID 集合 UUID（Init 时获取，写入外键引用）.
	// [EN] Collection UUID (acquired on Init; FK target for writes).
	collectionUUID string

	// vectorDimensions 向量维度（>0 时建定型列并校验写入）.
	// [EN] Vector dimension (typed column + write validation when > 0).
	vectorDimensions int

	// hnsw HNSW 索引配置（nil 为不建索引）.
	// [EN] HNSW index configuration (nil for no index).
	hnsw *hnswIndex
}

// New 构造向量库（连接池，URL 形态 postgres://user:pass@host/db）.
// [EN] Build a store (connection pool via postgres:// URL).
func New(ctx context.Context, connURL string, opts ...Option) (*Store, error) {
	pool, err := pgxpool.New(ctx, connURL)
	if err != nil {
		return nil, err
	}
	return NewWithConn(ctx, pool, opts...)
}

// NewWithConn 注入既有连接（*pgx.Conn 或 *pgxpool.Pool，生产推荐池）.
// [EN] Inject an existing connection (*pgx.Conn or *pgxpool.Pool; pool recommended).
func NewWithConn(ctx context.Context, c pgxConn, opts ...Option) (*Store, error) {
	s := &Store{
		db:              &pgxAdapter{c: c},
		collectionName:  DefaultCollectionName,
		embeddingTable:  DefaultEmbeddingTable,
		collectionTable: DefaultCollectionTable,
	}
	for _, o := range opts {
		o(s)
	}
	if err := s.Init(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// newWithDB 注入窄通道（测试专用）.
// [EN] Inject a narrow channel (testing only).
func newWithDB(d db, opts ...Option) *Store {
	s := &Store{
		db:              d,
		collectionName:  DefaultCollectionName,
		embeddingTable:  DefaultEmbeddingTable,
		collectionTable: DefaultCollectionTable,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Option 配置函数.
// [EN] Configuration function.
type Option func(*Store)

// WithCollectionName 设置集合名（langchaingo 存量集合直读）.
// [EN] Set the collection name.
func WithCollectionName(name string) Option {
	return func(s *Store) { s.collectionName = name }
}

// WithEmbeddingTable 设置嵌入表名.
// [EN] Set the embedding table name.
func WithEmbeddingTable(name string) Option {
	return func(s *Store) { s.embeddingTable = name }
}

// WithCollectionTable 设置集合表名.
// [EN] Set the collection table name.
func WithCollectionTable(name string) Option {
	return func(s *Store) { s.collectionTable = name }
}

// WithVectorDimensions 设置向量维度（建定型列 vector(N) 并校验写入）.
// [EN] Set the vector dimension (typed column vector(N) + write validation).
func WithVectorDimensions(d int) Option {
	return func(s *Store) { s.vectorDimensions = d }
}

// WithHNSWIndex 启用 HNSW 索引（m 每层连接数 / efConstruction 构建候选 /
// distanceFunction 距离算子类，见 Distance* 常量）.
// [EN] Enable an HNSW index.
func WithHNSWIndex(m, efConstruction int, distanceFunction string) Option {
	return func(s *Store) {
		s.hnsw = &hnswIndex{m: m, efConstruction: efConstruction, distanceFunction: distanceFunction}
	}
}

// Init 初始化：建扩展/双表/HNSW 索引，upsert 集合并取 UUID（幂等）.
// [EN] Initialize: extension/tables/HNSW, upsert collection, acquire UUID (idempotent).
func (s *Store) Init(ctx context.Context) error {
	for _, stmt := range s.buildDDL() {
		if err := s.db.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			return err
		}
	}
	if err := s.db.Exec(ctx,
		fmt.Sprintf(`INSERT INTO %s (uuid, name, cmetadata) VALUES($1, $2, $3)
	ON CONFLICT (name) DO UPDATE SET cmetadata = $3`, s.collectionTable),
		newUUID(), s.collectionName, nil,
	); err != nil {
		return err
	}
	rows, err := s.db.Query(ctx,
		fmt.Sprintf("SELECT uuid FROM %s WHERE name = $1 ORDER BY name LIMIT 1", s.collectionTable),
		s.collectionName,
	)
	if err != nil {
		return err
	}
	if len(rows) == 0 || len(rows[0]) == 0 {
		return fmt.Errorf("%w: collection %q missing after upsert", llmx.ErrInvalidRequest, s.collectionName)
	}
	s.collectionUUID = uuidString(rows[0][0])
	return nil
}

// AddDocuments 实现 llmx.VectorStore（pgx.Batch 单往返批量写入）.
// [EN] Implement llmx.VectorStore (batched writes via pgx.Batch).
func (s *Store) AddDocuments(ctx context.Context, docs []llmx.Document, vectors [][]float64) error {
	if len(docs) != len(vectors) {
		return llmx.ErrInvalidVectors
	}
	for _, v := range vectors {
		if len(v) == 0 || (s.vectorDimensions > 0 && len(v) != s.vectorDimensions) {
			return fmt.Errorf("%w: dim %d != typed %d", llmx.ErrInvalidVectors, len(v), s.vectorDimensions)
		}
	}

	sql := fmt.Sprintf(`INSERT INTO %s (uuid, document, embedding, cmetadata, collection_id)
	VALUES($1, $2, $3, $4, $5)`, s.embeddingTable)
	stmts := make([]statement, len(docs))
	for i, d := range docs {
		var meta any
		if d.Metadata != nil {
			b, err := json.Marshal(d.Metadata)
			if err != nil {
				return fmt.Errorf("%w: metadata: %v", llmx.ErrInvalidRequest, err)
			}
			meta = string(b)
		}
		stmts[i] = statement{sql: sql, args: []any{
			newUUID(), d.PageContent, encodeVectorText(vectors[i]), meta, s.collectionUUID,
		}}
	}
	return s.db.SendBatch(ctx, stmts)
}

// SimilaritySearch 实现 llmx.VectorStore（距离算子随 HNSW 自适应，过滤参数化）.
// [EN] Implement llmx.VectorStore (operator adapts to HNSW; parameterized filters).
func (s *Store) SimilaritySearch(ctx context.Context, query []float64, topK int, filters ...llmx.Filter) ([]llmx.Document, error) {
	if len(query) == 0 || (s.vectorDimensions > 0 && len(query) != s.vectorDimensions) {
		return nil, fmt.Errorf("%w: query dim %d != typed %d", llmx.ErrInvalidVectors, len(query), s.vectorDimensions)
	}
	sql, args := s.buildSearch(query, topK, filters)
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return rowsToDocuments(rows)
}

// 编译期断言：实现 llmx.VectorStore 契约.
// [EN] Compile-time assertion of the llmx.VectorStore contract.
var _ llmx.VectorStore = (*Store)(nil)
