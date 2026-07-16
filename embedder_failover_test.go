/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 23:36:54
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 23:36:54
 * @FilePath: \go-llmx\embedder_failover_test.go
 * @Description: Embedder 故障切换测试 —— 依序切换/不可切换错误透传/
 * 耗尽返回最后错误/空配置报错/向量一致性，fake 注入零网络
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEmbedder 固定向量/错误注入嵌入器.
// [EN] Fixed-vector / error-injecting embedder.
type fakeEmbedder struct {
	// vec 返回向量.
	// [EN] Returned vector.
	vec [][]float64

	// queryVec 单条查询向量.
	// [EN] Query vector.
	queryVec []float64

	// err 返回错误（非 nil 优先）.
	// [EN] Error to return (takes precedence).
	err error

	// calls 收到的调用计数.
	// [EN] Call count.
	calls int
}

// EmbedDocuments 实现 Embedder.
// [EN] Implement Embedder.
func (f *fakeEmbedder) EmbedDocuments(_ context.Context, _ []string) ([][]float64, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.vec, nil
}

// EmbedQuery 实现 Embedder.
// [EN] Implement Embedder.
func (f *fakeEmbedder) EmbedQuery(_ context.Context, _ string) ([]float64, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.queryVec, nil
}

// TestEmbedderFailover_SequentialFailover 验证依序切换（Docs/Query 双路径）.
// [EN] Verify sequential failover (both paths).
func TestEmbedderFailover_SequentialFailover(t *testing.T) {
	primary := &fakeEmbedder{err: ErrProviderUnavailable}
	backup := &fakeEmbedder{
		vec:      [][]float64{{1, 2}, {3, 4}},
		queryVec: []float64{5, 6},
	}
	f := NewEmbedderFailover(primary, backup)

	docs, err := f.EmbedDocuments(context.Background(), []string{"a", "b"})
	require.NoError(t, err)
	assert.Equal(t, [][]float64{{1, 2}, {3, 4}}, docs)

	q, err := f.EmbedQuery(context.Background(), "q")
	require.NoError(t, err)
	assert.Equal(t, []float64{5, 6}, q)
	assert.Equal(t, 2, primary.calls) // 主后端两次都被尝试
	assert.Equal(t, 2, backup.calls)
}

// TestEmbedderFailover_NonFailoverError 验证不可切换错误直接透传.
// [EN] Verify non-failover errors propagate.
func TestEmbedderFailover_NonFailoverError(t *testing.T) {
	bad := errors.New("invalid api key")
	primary := &fakeEmbedder{err: bad}
	backup := &fakeEmbedder{queryVec: []float64{1}}
	f := NewEmbedderFailover(primary, backup)

	_, err := f.EmbedQuery(context.Background(), "q")
	assert.ErrorIs(t, err, bad)
	assert.Equal(t, 0, backup.calls) // 不切换
}

// TestEmbedderFailover_Exhausted 验证全失败返回最后错误.
// [EN] Verify exhaustion returns the last error.
func TestEmbedderFailover_Exhausted(t *testing.T) {
	last := ErrAPIServerError
	f := NewEmbedderFailover(
		&fakeEmbedder{err: ErrProviderUnavailable},
		&fakeEmbedder{err: last},
	)
	_, err := f.EmbedQuery(context.Background(), "q")
	assert.ErrorIs(t, err, last)
}

// TestEmbedderFailover_Empty 验证空配置报错.
// [EN] Verify empty configuration errors.
func TestEmbedderFailover_Empty(t *testing.T) {
	f := NewEmbedderFailover()
	_, err := f.EmbedDocuments(context.Background(), nil)
	assert.ErrorIs(t, err, ErrNoEmbedders)
	_, err = f.EmbedQuery(context.Background(), "q")
	assert.ErrorIs(t, err, ErrNoEmbedders)
}

// TestEmbedderFailover_PrimaryHealthy 验证主后端健康零切换.
// [EN] Verify zero failover when primary is healthy.
func TestEmbedderFailover_PrimaryHealthy(t *testing.T) {
	primary := &fakeEmbedder{queryVec: []float64{9, 9}}
	backup := &fakeEmbedder{queryVec: []float64{1, 1}}
	f := NewEmbedderFailover(primary, backup)

	vec, err := f.EmbedQuery(context.Background(), "q")
	require.NoError(t, err)
	assert.Equal(t, []float64{9, 9}, vec)
	assert.Equal(t, 1, primary.calls)
	assert.Equal(t, 0, backup.calls)
}
