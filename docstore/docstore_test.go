/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-17 21:36:19
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-17 21:36:19
 * @FilePath: \go-llmx\docstore\docstore_test.go
 * @Description: 文档键值存储测试 —— 往返/未命中/覆盖/删除幂等/
 * 键序与副本隔离/并发安全.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package docstore

import (
	"context"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemory_SetGetRoundtrip(t *testing.T) {
	s := NewInMemory()
	doc := llmx.Document{PageContent: "正文内容", Metadata: map[string]any{"src": "a.md"}}

	require.NoError(t, s.Set(context.Background(), "k1", doc))
	got, err := s.Get(context.Background(), "k1")
	require.NoError(t, err)
	assert.Equal(t, doc, got)
}

func TestInMemory_GetNotFound(t *testing.T) {
	s := NewInMemory()

	_, err := s.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.ErrorContains(t, err, "missing")
}

func TestInMemory_SetOverwriteInPlace(t *testing.T) {
	s := NewInMemory()
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "k1", llmx.Document{PageContent: "v1"}))
	require.NoError(t, s.Set(ctx, "k1", llmx.Document{PageContent: "v2"}))

	got, err := s.Get(ctx, "k1")
	require.NoError(t, err)
	assert.Equal(t, "v2", got.PageContent)

	// 覆盖不新增键，插入序稳定
	assert.Equal(t, []string{"k1"}, s.Keys(ctx))
}

func TestInMemory_DeleteIdempotent(t *testing.T) {
	s := NewInMemory()
	ctx := context.Background()

	require.NoError(t, s.Set(ctx, "k1", llmx.Document{PageContent: "v"}))
	require.NoError(t, s.Set(ctx, "k2", llmx.Document{PageContent: "v"}))

	require.NoError(t, s.Delete(ctx, "k1"))
	_, err := s.Get(ctx, "k1")
	assert.ErrorIs(t, err, ErrNotFound)

	// 重复删除幂等成功，余键保序
	require.NoError(t, s.Delete(ctx, "k1"))
	assert.Equal(t, []string{"k2"}, s.Keys(ctx))
}

func TestInMemory_KeysInsertionOrderAndCopy(t *testing.T) {
	s := NewInMemory()
	ctx := context.Background()

	for _, k := range []string{"b", "a", "c"} {
		require.NoError(t, s.Set(ctx, k, llmx.Document{PageContent: "v"}))
	}
	assert.Equal(t, []string{"b", "a", "c"}, s.Keys(ctx))

	// 外部修改返回切片不影响内部状态
	keys := s.Keys(ctx)
	keys[0] = "hacked"
	assert.Equal(t, []string{"b", "a", "c"}, s.Keys(ctx))
}

func TestInMemory_ConcurrentAccess(t *testing.T) {
	s := NewInMemory()
	ctx := context.Background()
	var wg sync.WaitGroup

	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				key := string(rune('a' + g%26))
				_ = s.Set(ctx, key, llmx.Document{PageContent: "v"})
				_, _ = s.Get(ctx, key)
				_ = s.Keys(ctx)
				_ = s.Delete(ctx, key)
			}
		}(g)
	}
	wg.Wait()

	// 并发增删后状态自洽：Keys 与 Get 一致
	for _, k := range s.Keys(ctx) {
		_, err := s.Get(ctx, k)
		require.NoError(t, err)
	}
}
