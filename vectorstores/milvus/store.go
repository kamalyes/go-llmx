/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-13 22:51:07
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-13 23:07:26
 * @FilePath: \go-llmx\vectorstores\milvus\store.go
 * @Description: Milvus 向量库 —— RESTful v2 协议直连（复用核心 transport），
 * 免重量级 SDK 依赖. 集合 schema 与 langchaingo/milvus 同构（pk 自增 +
 * text/meta/vector），存量集合平滑读写；写入后刷盘可关，检索输出字段裁剪
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmilvus

import (
	"context"
	"fmt"
	"time"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/transport"
)

// poster 窄 POST 通道（*transport.Client 满足；测试注入零依赖）.
// [EN] Narrow POST channel (satisfied by *transport.Client).
type poster interface {
	PostJSON(ctx context.Context, url string, in, out any, headers map[string]string) error
}

// Store Milvus 向量库.
// [EN] Milvus vector store.
type Store struct {
	// post REST 通道.
	// [EN] REST channel.
	post poster

	// baseURL RESTful v2 根地址（含 /v2/vectordb）.
	// [EN] RESTful v2 root URL.
	baseURL string

	// headers 认证头（Bearer token / user:pass）.
	// [EN] Auth headers.
	headers map[string]string

	// collectionName 集合名（langchaingo 同名直读）.
	// [EN] Collection name.
	collectionName string

	// primaryField / textField / metaField / vectorField 字段名.
	// [EN] Field names.
	primaryField string
	textField    string
	metaField    string
	vectorField  string

	// dim 向量维度（存量集合 describe 自动识别）.
	// [EN] Vector dimension (auto-detected from describe).
	dim int

	// autoID 主键自增（langchaingo 同构）.
	// [EN] Auto primary key.
	autoID bool

	// maxTextLength VarChar 列上限.
	// [EN] VarChar column limit.
	maxTextLength int

	// metricType 度量类型（COSINE/IP/L2）.
	// [EN] Metric type.
	metricType string

	// skipFlush 写入后跳过刷盘（吞吐优先）.
	// [EN] Skip flush after insert (throughput-first).
	skipFlush bool
}

// New 构造向量库（RESTful v2 根地址 + Bearer 认证）.
// [EN] Build a store (RESTful v2 base URL + bearer auth).
func New(baseURL, token string, opts ...Option) *Store {
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	s := &Store{
		post:          transport.NewClient(transport.WithTimeout(30 * time.Second)),
		baseURL:       baseURL + apiVersion,
		headers:       headers,
		collectionName: DefaultCollectionName,
		primaryField:   DefaultPrimaryField,
		textField:      DefaultTextField,
		metaField:      DefaultMetaField,
		vectorField:    DefaultVectorField,
		autoID:         true,
		maxTextLength:  defaultMaxLength,
		metricType:     MetricCosine,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// newWithPoster 注入窄通道与根地址（测试专用）.
// [EN] Inject a narrow channel and root URL (testing only).
func newWithPoster(p poster, root string, opts ...Option) *Store {
	s := &Store{
		post:           p,
		baseURL:        root,
		collectionName: DefaultCollectionName,
		primaryField:   DefaultPrimaryField,
		textField:      DefaultTextField,
		metaField:      DefaultMetaField,
		vectorField:    DefaultVectorField,
		autoID:         true,
		maxTextLength:  defaultMaxLength,
		metricType:     MetricCosine,
		headers:        map[string]string{},
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

// WithFields 覆盖字段名（自定义 schema 的存量集合）.
// [EN] Override field names (custom-schema collections).
func WithFields(primary, text, meta, vector string) Option {
	return func(s *Store) {
		s.primaryField, s.textField, s.metaField, s.vectorField = primary, text, meta, vector
	}
}

// WithDimensions 设置向量维度（建集合用；存量集合 describe 自动识别）.
// [EN] Set the vector dimension (creation; auto-detected on existing).
func WithDimensions(d int) Option {
	return func(s *Store) { s.dim = d }
}

// WithMetricType 设置度量类型（COSINE/IP/L2）.
// [EN] Set the metric type.
func WithMetricType(m string) Option {
	return func(s *Store) { s.metricType = m }
}

// WithSkipFlush 写入后跳过刷盘（吞吐优先， durability 交由服务端策略）.
// [EN] Skip flush after insert.
func WithSkipFlush() Option {
	return func(s *Store) { s.skipFlush = true }
}

// WithTimeout 设置 REST 超时.
// [EN] Set the REST timeout.
func WithTimeout(d time.Duration) Option {
	return func(s *Store) {
		s.post = transport.NewClient(transport.WithTimeout(d))
	}
}

// Init 初始化集合（幂等）：存在则 describe 对齐维度与加载态，否则建集合.
// [EN] Initialize the collection (idempotent).
func (s *Store) Init(ctx context.Context) error {
	has := &wireHasResponse{}
	if err := s.post.PostJSON(ctx, s.baseURL+pathCollectionsHas,
		&wireHasRequest{CollectionName: s.collectionName}, has, s.headers); err != nil {
		return err
	}
	if err := has.asError(); err != nil {
		return err
	}

	if has.Data != nil && has.Data.Has {
		descr := &wireDescribeResponse{}
		if err := s.post.PostJSON(ctx, s.baseURL+pathCollectionsDescr,
			&wireDescribeRequest{CollectionName: s.collectionName}, descr, s.headers); err != nil {
			return err
		}
		if err := descr.asError(); err != nil {
			return err
		}
		dim, err := vectorDimFromDescribe(descr.Data, s.vectorField)
		if err != nil {
			return err
		}
		s.dim = dim
		if descr.Data.Load != "Loaded" {
			return s.load(ctx)
		}
		return nil
	}

	if s.dim <= 0 {
		return fmt.Errorf("%w: dimension required to create collection", llmx.ErrInvalidRequest)
	}
	created := &wireResponse{}
	if err := s.post.PostJSON(ctx, s.baseURL+pathCollectionsCreate,
		buildCreateRequest(s), created, s.headers); err != nil {
		return err
	}
	if err := created.asError(); err != nil {
		return err
	}
	return s.load(ctx)
}

// load 加载集合进内存.
// [EN] Load the collection into memory.
func (s *Store) load(ctx context.Context) error {
	resp := &wireResponse{}
	err := s.post.PostJSON(ctx, s.baseURL+pathCollectionsLoad,
		&wireLoadRequest{CollectionName: s.collectionName}, resp, s.headers)
	if err != nil {
		return err
	}
	return resp.asError()
}

// AddDocuments 实现 llmx.VectorStore（行式批量写入 + 可选刷盘）.
// [EN] Implement llmx.VectorStore (row-based batch insert + optional flush).
func (s *Store) AddDocuments(ctx context.Context, docs []llmx.Document, vectors [][]float64) error {
	if len(docs) != len(vectors) {
		return llmx.ErrInvalidVectors
	}
	if len(docs) == 0 {
		return nil
	}
	dim := s.dim
	if dim == 0 && len(vectors[0]) > 0 {
		dim = len(vectors[0])
	}
	for _, v := range vectors {
		if len(v) != dim {
			return fmt.Errorf("%w: inconsistent dim %d != %d", llmx.ErrInvalidVectors, len(v), dim)
		}
	}

	rows := make([]map[string]any, len(docs))
	for i, d := range docs {
		row := map[string]any{
			s.textField:   d.PageContent,
			s.vectorField: vectors[i],
		}
		if d.Metadata != nil {
			row[s.metaField] = d.Metadata
		}
		rows[i] = row
	}

	resp := &wireInsertResponse{}
	if err := s.post.PostJSON(ctx, s.baseURL+pathEntitiesInsert,
		&wireInsertRequest{CollectionName: s.collectionName, Data: rows}, resp, s.headers); err != nil {
		return err
	}
	if err := resp.asError(); err != nil {
		return err
	}
	if resp.Data == nil || resp.Data.InsertCount != len(docs) {
		return fmt.Errorf("%w: insert count mismatch: %+v", llmx.ErrInvalidRequest, resp.Data)
	}

	if s.skipFlush {
		return nil
	}
	flushed := &wireResponse{}
	if err := s.post.PostJSON(ctx, s.baseURL+pathEntitiesFlush,
		&wireFlushRequest{CollectionNames: []string{s.collectionName}}, flushed, s.headers); err != nil {
		return err
	}
	return flushed.asError()
}

// SimilaritySearch 实现 llmx.VectorStore（KNN + 过滤表达式 + 输出裁剪）.
// [EN] Implement llmx.VectorStore (KNN + filter expression + output trimming).
func (s *Store) SimilaritySearch(ctx context.Context, query []float64, topK int, filters ...llmx.Filter) ([]llmx.Document, error) {
	if len(query) == 0 || (s.dim > 0 && len(query) != s.dim) {
		return nil, fmt.Errorf("%w: query dim %d != collection %d", llmx.ErrInvalidVectors, len(query), s.dim)
	}
	resp := &wireSearchResponse{}
	if err := s.post.PostJSON(ctx, s.baseURL+pathEntitiesSearch,
		buildSearchRequest(s, query, topK, filters), resp, s.headers); err != nil {
		return nil, err
	}
	if err := resp.asError(); err != nil {
		return nil, err
	}
	return rowsToDocuments(resp.Data, s.textField, s.metaField), nil
}

// 编译期断言：实现 llmx.VectorStore 契约.
// [EN] Compile-time assertion of the llmx.VectorStore contract.
var _ llmx.VectorStore = (*Store)(nil)
