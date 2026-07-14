/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-14 21:33:08
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-14 22:05:19
 * @FilePath: \go-llmx\vectorstores\qdrant\store.go
 * @Description: Qdrant 向量库 —— REST 直连（复用核心 transport），
 * 免 gRPC 客户端依赖. payload 与 langchaingo/qdrant 同构
 * （page_content/metadata），存量集合平滑读写；写入走 batch upsert
 * 单 RTT，检索 with_payload 取回裁剪
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcqdrant

import (
	"context"
	"fmt"
	"time"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/transport"
)

// doer 窄方法通道（*transport.Client 满足；测试注入零依赖）.
// [EN] Narrow method channel (satisfied by *transport.Client).
type doer interface {
	// DoJSON 发送任意方法 JSON 请求并解析响应.
	// [EN] Send a method-agnostic JSON request and decode the response.
	DoJSON(ctx context.Context, method, url string, in, out any, headers map[string]string) error
}

// Store Qdrant 向量库.
// [EN] Qdrant vector store.
type Store struct {
	// do REST 通道.
	// [EN] REST channel.
	do doer

	// baseURL REST 根地址.
	// [EN] REST root URL.
	baseURL string

	// headers 认证头（Bearer api-key）.
	// [EN] Auth headers (bearer api-key).
	headers map[string]string

	// collectionName 集合名（langchaingo 存量集合直读）.
	// [EN] Collection name.
	collectionName string

	// dim 向量维度（存量集合 info 自动识别）.
	// [EN] Vector dimension (auto-detected from info).
	dim int

	// distance 距离度量（Cosine/Euclid/Dot）.
	// [EN] Distance metric.
	distance string

	// waitUpsert 写入等待落盘（默认真，强一致）.
	// [EN] Wait for upsert durability (default true).
	waitUpsert bool
}

// New 构造向量库（REST 地址 + Bearer api-key）.
// [EN] Build a store (REST base URL + bearer api-key).
func New(baseURL, apiKey string, opts ...Option) *Store {
	headers := map[string]string{}
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}
	s := &Store{
		do:             transport.NewClient(transport.WithTimeout(30 * time.Second)),
		baseURL:        baseURL,
		headers:        headers,
		distance:       DistanceCosine,
		waitUpsert:     true,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// newWithDoer 注入窄通道与根地址（测试专用）.
// [EN] Inject a narrow channel and root URL (testing only).
func newWithDoer(d doer, root string, opts ...Option) *Store {
	s := &Store{
		do:             d,
		baseURL:        root,
		headers:        map[string]string{},
		distance:       DistanceCosine,
		waitUpsert:     true,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Option 配置函数.
// [EN] Configuration function.
type Option func(*Store)

// WithCollectionName 设置集合名（必填；langchaingo 存量集合直读）.
// [EN] Set the collection name (required).
func WithCollectionName(name string) Option {
	return func(s *Store) { s.collectionName = name }
}

// WithDimensions 设置向量维度（建集合用；存量集合 info 自动识别）.
// [EN] Set the vector dimension (creation; auto-detected on existing).
func WithDimensions(d int) Option {
	return func(s *Store) { s.dim = d }
}

// WithDistance 设置距离度量（Cosine/Euclid/Dot）.
// [EN] Set the distance metric.
func WithDistance(d string) Option {
	return func(s *Store) { s.distance = d }
}

// WithNoWait 写入不等落盘（吞吐优先，durability 交由服务端策略）.
// [EN] Skip upsert durability wait (throughput-first).
func WithNoWait() Option {
	return func(s *Store) { s.waitUpsert = false }
}

// WithTimeout 设置 REST 超时.
// [EN] Set the REST timeout.
func WithTimeout(d time.Duration) Option {
	return func(s *Store) {
		s.do = transport.NewClient(transport.WithTimeout(d))
	}
}

// collectionURL 集合资源地址.
// [EN] Collection resource URL.
func (s *Store) collectionURL() string {
	return s.baseURL + pathCollections + "/" + s.collectionName
}

// upsertURL 写入地址（wait=true 强一致）.
// [EN] Upsert URL (wait=true for strong durability).
func (s *Store) upsertURL() string {
	if !s.waitUpsert {
		return s.collectionURL() + pathPoints
	}
	return s.collectionURL() + pathPoints + "?wait=true"
}

// Init 初始化集合（幂等）：存在则 info 对齐维度，否则建集合.
// [EN] Initialize the collection (idempotent).
func (s *Store) Init(ctx context.Context) error {
	if s.collectionName == "" {
		return fmt.Errorf("%w: collection name required", llmx.ErrInvalidRequest)
	}
	info := &wireCollectionInfo{}
	err := s.do.DoJSON(ctx, transport.MethodGet, s.collectionURL(), nil, info, s.headers)
	if err == nil {
		if info.Result.Config.Params.Vectors != nil && info.Result.Config.Params.Vectors.Size > 0 {
			s.dim = info.Result.Config.Params.Vectors.Size
		}
		return nil
	}
	if !isNotFound(err) {
		return err
	}

	if s.dim <= 0 {
		return fmt.Errorf("%w: dimension required to create collection", llmx.ErrInvalidRequest)
	}
	created := &wireResultResponse{}
	if err := s.do.DoJSON(ctx, transport.MethodPut, s.collectionURL(),
		buildCreateRequest(s.dim, s.distance), created, s.headers); err != nil {
		return err
	}
	if !created.ok() {
		return fmt.Errorf("%w: collection not created", llmx.ErrInvalidRequest)
	}
	return nil
}

// AddDocuments 实现 llmx.VectorStore（batch upsert 单 RTT）.
// [EN] Implement llmx.VectorStore (batch upsert in one round-trip).
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

	upserted := &wireResultResponse{}
	if err := s.do.DoJSON(ctx, transport.MethodPut, s.upsertURL(),
		buildUpsertRequest(docs, vectors), upserted, s.headers); err != nil {
		return err
	}
	if !upserted.ok() {
		return fmt.Errorf("%w: upsert not completed", llmx.ErrInvalidRequest)
	}
	return nil
}

// SimilaritySearch 实现 llmx.VectorStore（KNN + 嵌套 key 过滤）.
// [EN] Implement llmx.VectorStore (KNN + nested-key filter).
func (s *Store) SimilaritySearch(ctx context.Context, query []float64, topK int, filters ...llmx.Filter) ([]llmx.Document, error) {
	if len(query) == 0 || (s.dim > 0 && len(query) != s.dim) {
		return nil, fmt.Errorf("%w: query dim %d != collection %d", llmx.ErrInvalidVectors, len(query), s.dim)
	}
	result := &wireSearchResult{}
	if err := s.do.DoJSON(ctx, transport.MethodPost, s.collectionURL()+pathSearch,
		buildSearchRequest(query, topK, filters), result, s.headers); err != nil {
		return nil, err
	}
	return hitsToDocuments(result.Result), nil
}

// 编译期断言：实现 llmx.VectorStore 契约.
// [EN] Compile-time assertion of the llmx.VectorStore contract.
var _ llmx.VectorStore = (*Store)(nil)
