/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-16 23:31:08
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-16 23:31:08
 * @FilePath: \go-llmx\embedder_failover.go
 * @Description: Embedder 故障切换 —— 同模型多 Key 限流/网络故障时切换，
 * 向量空间保持一致（跨模型切换会破坏索引一致性，不在本组件语义内）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"context"
	"errors"
)

// ErrNoEmbedders Embedder 路由未配置任何后端.
// [EN] Embedder router has no backends configured.
var ErrNoEmbedders = errors.New("llmx: no embedders configured")

// EmbedderFailover 依序故障切换嵌入器：当前后端失败（不可达 / 5xx / 空响应）
// 时自动切到下一个后端重试。定位同模型多 Key 容灾——切换不改变向量空间，
// 存量索引与查询向量保持一致.
// [EN] Sequential failover embedder: retry with the next backend on failure.
// Positioned for multi-key resilience of the same model — vector space intact.
type EmbedderFailover struct {
	// embedders 依序尝试的后端列表（至少一个）.
	// [EN] Backends tried in order (at least one).
	embedders []Embedder
}

// NewEmbedderFailover 构造依序故障切换嵌入器.
// [EN] Build a sequential failover embedder.
func NewEmbedderFailover(embedders ...Embedder) *EmbedderFailover {
	return &EmbedderFailover{embedders: embedders}
}

// EmbedDocuments 实现 Embedder（依序尝试后端直至成功或耗尽）.
// [EN] Implement Embedder (try backends until success or exhaustion).
func (f *EmbedderFailover) EmbedDocuments(ctx context.Context, texts []string) ([][]float64, error) {
	if len(f.embedders) == 0 {
		return nil, ErrNoEmbedders
	}
	var lastErr error
	for _, e := range f.embedders {
		vecs, err := e.EmbedDocuments(ctx, texts)
		if err == nil {
			return vecs, nil
		}
		if !failoverOf(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// EmbedQuery 实现 Embedder.
// [EN] Implement Embedder.
func (f *EmbedderFailover) EmbedQuery(ctx context.Context, text string) ([]float64, error) {
	if len(f.embedders) == 0 {
		return nil, ErrNoEmbedders
	}
	var lastErr error
	for _, e := range f.embedders {
		vec, err := e.EmbedQuery(ctx, text)
		if err == nil {
			return vec, nil
		}
		if !failoverOf(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// 编译期断言：实现 Embedder 契约.
// [EN] Compile-time assertion.
var _ Embedder = (*EmbedderFailover)(nil)
