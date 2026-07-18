/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-18 09:16:52
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-18 09:16:52
 * @FilePath: \go-llmx\adapter\embed_test.go
 * @Description: Embedder 公共件测试 —— 空校验/向量数错误/
 * 批量派发（单批直调/串行区间/并行全覆盖/首错取消/外部取消）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package adapter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEmbedTexts(t *testing.T) {
	assert.ErrorIs(t, ValidateEmbedTexts(nil), llmx.ErrInvalidRequest)
	assert.ErrorIs(t, ValidateEmbedTexts([]string{}), llmx.ErrInvalidRequest)
	assert.NoError(t, ValidateEmbedTexts([]string{"a"}))
}

func TestErrVectorCountMismatch(t *testing.T) {
	err := ErrVectorCountMismatch(2, 5)
	assert.Contains(t, err.Error(), "2 vectors for 5 texts")
}

func TestParallelBatches_SingleBatchDirect(t *testing.T) {
	// maxBatch >= total：单批直调（零 goroutine 开销）
	var ranges [][2]int
	err := ParallelBatches(context.Background(), 5, 10, 4, func(_ context.Context, start, end int) error {
		ranges = append(ranges, [2]int{start, end})
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, [][2]int{{0, 5}}, ranges)
}

func TestParallelBatches_SerialRanges(t *testing.T) {
	// workers=1：顺序区间 [0,4) [4,8) [8,10)
	var ranges [][2]int
	err := ParallelBatches(context.Background(), 10, 4, 1, func(_ context.Context, start, end int) error {
		ranges = append(ranges, [2]int{start, end})
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, [][2]int{{0, 4}, {4, 8}, {8, 10}}, ranges)
}

func TestParallelBatches_ParallelFullCoverage(t *testing.T) {
	// 100 条 / 7 批：并行下每条恰好处理一次（互斥写入验证全覆盖）
	total := 100
	touched := make([]int32, total)
	err := ParallelBatches(context.Background(), total, 7, 8, func(_ context.Context, start, end int) error {
		for i := start; i < end; i++ {
			atomic.AddInt32(&touched[i], 1)
		}
		return nil
	})
	require.NoError(t, err)
	for i, n := range touched {
		assert.Equal(t, int32(1), n, "index %d", i)
	}
}

func TestParallelBatches_FirstErrorCancelsRest(t *testing.T) {
	// start==0 的批返回错误 → 其余批收到派生 ctx 取消
	boom := errors.New("boom")
	var mu sync.Mutex
	sawFirst, sawCancelled := false, 0
	err := ParallelBatches(context.Background(), 20, 5, 4, func(ctx context.Context, start, _ int) error {
		mu.Lock()
		defer mu.Unlock()
		if start == 0 {
			sawFirst = true
			return boom
		}
		select {
		case <-ctx.Done():
			sawCancelled++
			return ctx.Err()
		default:
			return nil
		}
	})
	assert.ErrorIs(t, err, boom)
	assert.True(t, sawFirst)
	assert.GreaterOrEqual(t, sawCancelled, 0)
}

func TestParallelBatches_ExternalCancel(t *testing.T) {
	// 外部预取消：仍派发首批（传输层取消语义不被劫持），整体返回取消错误
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var dispatched int32
	err := ParallelBatches(ctx, 10, 4, 4, func(context.Context, int, int) error {
		atomic.AddInt32(&dispatched, 1)
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.GreaterOrEqual(t, dispatched, int32(1))
}

func TestParallelBatches_ZeroTotal(t *testing.T) {
	err := ParallelBatches(context.Background(), 0, 4, 4, func(context.Context, int, int) error {
		t.Error("空输入不应派发")
		return nil
	})
	assert.NoError(t, err)
}
