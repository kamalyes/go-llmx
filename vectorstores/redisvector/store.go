/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-12 21:19:38
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-12 22:07:41
 * @FilePath: \go-llmx\vectorstores\redisvector\store.go
 * @Description: Redis 向量库（RediSearch）—— go-redis v9 直连 FT.* 命令.
 * 数据格式与 langchaingo/redisvector 完全对齐（content/content_vector 字段、
 * doc:<index>: 键前缀、元数据平铺），存量索引平滑读写；
 * 批量写入走 pipeline 单 RTT，检索 RETURN 裁剪跳过向量 blob 回传
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcredisvector

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"

	llmx "github.com/kamalyes/go-llmx"
)

// commandRunner 窄命令接口（go-redis 适配器满足；测试注入零依赖）.
// [EN] Narrow command interface (satisfied by the go-redis adapter).
type commandRunner interface {
	// Do 单命令执行，返回解码结果.
	// [EN] Execute one command, returning the decoded result.
	Do(ctx context.Context, args ...any) (any, error)

	// PipelineDo 批量命令单 RTT 执行.
	// [EN] Execute batched commands in one round-trip.
	PipelineDo(ctx context.Context, batches [][]any) error
}

// goRedisAdapter 将 *redis.Client 适配为窄接口.
// [EN] Adapt *redis.Client to the narrow interface.
type goRedisAdapter struct {
	c *redis.Client
}

// Do 实现 commandRunner.
// [EN] Implement commandRunner.
func (a *goRedisAdapter) Do(ctx context.Context, args ...any) (any, error) {
	return a.c.Do(ctx, args...).Result()
}

// PipelineDo 实现 commandRunner（一次 RTT 写入全部批次）.
// [EN] Implement commandRunner (all batches in one RTT).
func (a *goRedisAdapter) PipelineDo(ctx context.Context, batches [][]any) error {
	pipe := a.c.Pipeline()
	for _, args := range batches {
		pipe.Do(ctx, args...)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Store Redis 向量库.
// [EN] Redis vector store.
type Store struct {
	// runner 命令通道.
	// [EN] Command channel.
	runner commandRunner

	// index RediSearch 索引名.
	// [EN] RediSearch index name.
	index string

	// prefix HASH 键前缀（默认 doc:<index>，langchaingo 同构）.
	// [EN] HASH key prefix (default doc:<index>).
	prefix string

	// dim 向量维度（须与索引 DIM 一致）.
	// [EN] Vector dimension.
	dim int

	// mu 保护 knownFields.
	// [EN] Guards knownFields.
	mu sync.RWMutex

	// knownFields 已见元数据字段（检索 RETURN 裁剪依据）.
	// [EN] Known metadata fields (for RETURN trimming).
	knownFields map[string]struct{}
}

// New 构造向量库（标准单节点地址；复杂拓扑用 NewWithClient 注入）.
// [EN] Build a store (single-node address; use NewWithClient for complex topologies).
func New(addr string, opts ...Option) *Store {
	return NewWithClient(redis.NewClient(&redis.Options{Addr: addr}), opts...)
}

// NewWithClient 注入既有客户端（连接池/集群/哨兵复用，生产推荐）.
// [EN] Inject an existing client (pool/cluster/sentinel reuse; production choice).
func NewWithClient(c *redis.Client, opts ...Option) *Store {
	s := &Store{
		runner:      &goRedisAdapter{c: c},
		index:       DefaultIndex,
		prefix:      "doc:" + DefaultIndex,
		dim:         DefaultDimensions,
		knownFields: map[string]struct{}{},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// newWithRunner 注入窄命令通道（测试专用）.
// [EN] Inject a narrow command channel (testing only).
func newWithRunner(r commandRunner, opts ...Option) *Store {
	s := &Store{
		runner:      r,
		index:       DefaultIndex,
		prefix:      "doc:" + DefaultIndex,
		dim:         DefaultDimensions,
		knownFields: map[string]struct{}{},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Option 配置函数.
// [EN] Configuration function.
type Option func(*Store)

// WithIndex 设置索引名（键前缀随之派生 doc:<index>，langchaingo 存量索引直读）.
// [EN] Set the index name (key prefix derives as doc:<index>).
func WithIndex(name string) Option {
	return func(s *Store) {
		s.index = name
		s.prefix = "doc:" + name
	}
}

// WithPrefix 覆盖键前缀（自定义前缀的存量索引）.
// [EN] Override the key prefix.
func WithPrefix(p string) Option {
	return func(s *Store) { s.prefix = p }
}

// WithDimensions 设置向量维度（须与索引 DIM 一致）.
// [EN] Set the vector dimension.
func WithDimensions(d int) Option {
	return func(s *Store) { s.dim = d }
}

// WithFilterFields 声明过滤字段集（建 TAG 索引；检索时已知字段自动并入 RETURN）.
// [EN] Declare filter fields (TAG-indexed; merged into RETURN on search).
func WithFilterFields(fields ...string) Option {
	return func(s *Store) {
		for _, f := range fields {
			s.knownFields[f] = struct{}{}
		}
	}
}

// InitIndex 创建索引（幂等：already exists 静默通过）.
// [EN] Create the index (idempotent: already-exists is silent).
func (s *Store) InitIndex(ctx context.Context) error {
	_, err := s.runner.Do(ctx, buildCreateArgs(s.index, s.prefix, s.dim, s.snapshotFields())...)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

// AddDocuments 实现 llmx.VectorStore.
// [EN] Implement llmx.VectorStore.
//
// HSET：content + content_vector(FLOAT32 blob) + 元数据平铺（langchaingo 同构）；
// 批量走 pipeline 单 RTT；ids/keys 元数据语义优先作主键
func (s *Store) AddDocuments(ctx context.Context, docs []llmx.Document, vectors [][]float64) error {
	if len(docs) != len(vectors) {
		return llmx.ErrInvalidVectors
	}
	for _, v := range vectors {
		if len(v) != s.dim {
			return fmt.Errorf("%w: dim %d != index %d", llmx.ErrInvalidVectors, len(v), s.dim)
		}
	}

	batches := make([][]any, len(docs))
	fields := make([]string, 0, 8)
	for i, d := range docs {
		args := make([]any, 0, 6+len(d.Metadata)*2)
		args = append(args, "HSET", s.docKey(d.Metadata), fieldContent, d.PageContent, fieldVector, encodeVector(vectors[i]))
		for k, v := range d.Metadata {
			if v == nil {
				continue
			}
			args = append(args, k, strValue(v))
			fields = append(fields, k)
		}
		batches[i] = args
	}
	s.rememberFields(fields)
	return s.runner.PipelineDo(ctx, batches)
}

// SimilaritySearch 实现 llmx.VectorStore（KNN 余弦 + TAG 过滤 + RETURN 裁剪）.
// [EN] Implement llmx.VectorStore (KNN cosine + TAG filters + RETURN trimming).
func (s *Store) SimilaritySearch(ctx context.Context, query []float64, topK int, filters ...llmx.Filter) ([]llmx.Document, error) {
	if len(query) != s.dim {
		return nil, fmt.Errorf("%w: query dim %d != index %d", llmx.ErrInvalidVectors, len(query), s.dim)
	}
	args := buildSearchArgs(s.index, buildKNNQuery(topK, filters), query, topK, s.snapshotFields())
	v, err := s.runner.Do(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseSearchResponse(v), nil
}

// docKey 生成文档主键（langchaingo 语义：metadata ids/keys 优先，否则随机 hex）.
// [EN] Generate a document key (langchaingo semantics: ids/keys, else random hex).
func (s *Store) docKey(meta map[string]any) string {
	for _, k := range []string{"ids", "keys"} {
		if v, ok := meta[k]; ok {
			return s.prefix + ":" + strValue(v)
		}
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return s.prefix + ":" + hex.EncodeToString(b)
}

// rememberFields 记录写入路径见过的元数据字段（读锁外收集，写锁内合并）.
// [EN] Record metadata fields seen on the write path.
func (s *Store) rememberFields(fields []string) {
	if len(fields) == 0 {
		return
	}
	s.mu.Lock()
	for _, f := range fields {
		s.knownFields[f] = struct{}{}
	}
	s.mu.Unlock()
}

// snapshotFields 已知元数据字段快照（过滤字段已含其中）.
// [EN] Snapshot of known metadata fields.
func (s *Store) snapshotFields() []string {
	s.mu.RLock()
	fields := make([]string, 0, len(s.knownFields))
	for f := range s.knownFields {
		fields = append(fields, f)
	}
	s.mu.RUnlock()
	return fields
}

// 编译期断言：实现 llmx.VectorStore 契约.
// [EN] Compile-time assertion of the llmx.VectorStore contract.
var _ llmx.VectorStore = (*Store)(nil)
