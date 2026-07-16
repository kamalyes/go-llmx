/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 09:52:13
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 10:01:26
 * @FilePath: \go-llmx\failover_test.go
 * @Description: 路由组件测试 —— Failover 依序切换/流式语义、Random、
 * RoundRobin 分发与并发安全，FakeModel 注入零网络依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFailover_SequentialSuccess 验证主模型正常时不触发切换.
// [EN] Verify no failover when the primary is healthy.
func TestFailover_SequentialSuccess(t *testing.T) {
	primary := NewFakeModel("from primary")
	backup := NewFakeModel("from backup")
	f := NewFailover(primary, backup)

	out, err := Generate(context.Background(), f, "hi")
	require.NoError(t, err)
	assert.Equal(t, "from primary", out)
}

// TestFailover_SwitchesOnUnavailable 验证不可达/5xx/空响应触发切换.
// [EN] Verify failover on unreachable / 5xx / empty response.
func TestFailover_SwitchesOnUnavailable(t *testing.T) {
	for _, err := range []error{ErrProviderUnavailable, ErrAPIServerError, ErrEmptyResponse} {
		primary := NewFakeModel()
		primary.Err = err
		backup := NewFakeModel("from backup")
		f := NewFailover(primary, backup)

		out, err := Generate(context.Background(), f, "hi")
		require.NoError(t, err, "should failover on %v", err)
		assert.Equal(t, "from backup", out)
	}
}

// TestFailover_NonFailoverErrorsPropagate 验证参数/认证错误直接透传.
// [EN] Verify invalid-request / auth errors propagate without failover.
func TestFailover_NonFailoverErrorsPropagate(t *testing.T) {
	for _, err := range []error{ErrInvalidRequest, ErrUnauthorized} {
		primary := NewFakeModel()
		primary.Err = err
		backup := NewFakeModel("from backup")
		f := NewFailover(primary, backup)

		_, genErr := Generate(context.Background(), f, "hi")
		assert.ErrorIs(t, genErr, err, "should propagate %v", err)
	}
}

// TestFailover_AllBackendsFail 验证全部后端失败时返回最后一个错误.
// [EN] Verify the last error returns when all backends fail.
func TestFailover_AllBackendsFail(t *testing.T) {
	a, b := NewFakeModel(), NewFakeModel()
	a.Err, b.Err = ErrProviderUnavailable, ErrAPIServerError
	f := NewFailover(a, b)

	_, err := Generate(context.Background(), f, "hi")
	assert.ErrorIs(t, err, ErrAPIServerError)
}

// TestFailover_EmptyConfig 验证空配置报 ErrNoModels.
// [EN] Verify ErrNoModels on empty config.
func TestFailover_EmptyConfig(t *testing.T) {
	f := NewFailover()
	_, err := Generate(context.Background(), f, "hi")
	assert.ErrorIs(t, err, ErrNoModels)
}

// TestFailover_StreamSwitchesBeforeFirstChunk 验证流式首 chunk 前切换.
// [EN] Verify streaming failover before the first chunk.
func TestFailover_StreamSwitchesBeforeFirstChunk(t *testing.T) {
	primary := NewFakeModel()
	primary.Err = ErrProviderUnavailable
	backup := NewFakeModel("stream-ok")
	f := NewFailover(primary, backup)

	var sb strings.Builder
	resp, err := f.StreamGenerateContent(context.Background(),
		[]Message{User("hi")},
		func(c *Chunk) error {
			sb.WriteString(c.Content)
			return nil
		})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, sb.String(), "stream-ok")
}

// TestFailover_StreamNoSwitchAfterFirstChunk 验证首 chunk 后中断透传（防重复输出）.
// [EN] Verify mid-stream errors propagate (no duplicated output).
func TestFailover_StreamNoSwitchAfterFirstChunk(t *testing.T) {
	primary := NewFakeModel("partial")
	primary.StreamErr = ErrAPIServerError
	backup := NewFakeModel("backup")
	f := NewFailover(primary, backup)

	var sb strings.Builder
	_, err := f.StreamGenerateContent(context.Background(),
		[]Message{User("hi")},
		func(c *Chunk) error {
			sb.WriteString(c.Content)
			return nil
		})
	require.Error(t, err)
	// 只有 primary 的 partial，无 backup 重复内容
	// [EN] Only primary's partial output; no backup duplication.
	assert.Equal(t, "partial", sb.String())
}

// TestRandomModel_PicksAnyBackend 验证随机路由命中后端集合.
// [EN] Verify random routing hits configured backends.
func TestRandomModel_PicksAnyBackend(t *testing.T) {
	a := NewFakeModel("a")
	b := NewFakeModel("b")
	r := NewRandomModel(a, b)

	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		out, err := Generate(context.Background(), r, "hi")
		require.NoError(t, err)
		seen[out] = true
	}
	assert.True(t, seen["a"] && seen["b"], "both backends should be hit")
}

// TestRandomModel_EmptyConfig 验证空配置报 ErrNoModels.
// [EN] Verify ErrNoModels on empty config.
func TestRandomModel_EmptyConfig(t *testing.T) {
	r := NewRandomModel()
	_, err := Generate(context.Background(), r, "hi")
	assert.ErrorIs(t, err, ErrNoModels)
}

// TestRandomModel_Streaming 验证随机路由流式透传.
// [EN] Verify random routing with streaming.
func TestRandomModel_Streaming(t *testing.T) {
	m := NewFakeModel("s")
	m.StreamTexts = []string{"s"}
	r := NewRandomModel(m)

	resp, err := r.StreamGenerateContent(context.Background(),
		[]Message{User("hi")}, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// TestRoundRobinModel_RotatesInOrder 验证轮询顺序与循环.
// [EN] Verify round-robin order and wrap-around.
func TestRoundRobinModel_RotatesInOrder(t *testing.T) {
	a := NewFakeModel("a")
	b := NewFakeModel("b")
	r := NewRoundRobinModel(a, b)

	var order []string
	for i := 0; i < 5; i++ {
		out, err := Generate(context.Background(), r, "hi")
		require.NoError(t, err)
		order = append(order, out)
	}
	assert.Equal(t, []string{"a", "b", "a", "b", "a"}, order)
}

// TestRoundRobinModel_ConcurrentRotation 验证并发调用无锁分发.
// [EN] Verify lock-free distribution under concurrency.
func TestRoundRobinModel_ConcurrentRotation(t *testing.T) {
	a := NewFakeModel("a")
	b := NewFakeModel("b")
	r := NewRoundRobinModel(a, b)

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		aCnt, bCnt int
	)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := Generate(context.Background(), r, "hi")
			if err == nil {
				mu.Lock()
				if out == "a" {
					aCnt++
				} else {
					bCnt++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 100, aCnt+bCnt, "all calls should succeed")
}

// TestFailover_UnknownErrorPropagates 验证未知错误类型透传（保守不切换）.
// [EN] Verify unknown error types propagate (conservatively no failover).
func TestFailover_UnknownErrorPropagates(t *testing.T) {
	primary := NewFakeModel()
	primary.Err = errors.New("weird provider error")
	backup := NewFakeModel("from backup")
	f := NewFailover(primary, backup)

	_, err := Generate(context.Background(), f, "hi")
	assert.Error(t, err)
	assert.EqualError(t, err, "weird provider error")
}
