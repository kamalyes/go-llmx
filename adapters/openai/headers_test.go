/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-17 10:22:37
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-17 10:22:37
 * @FilePath: \go-llmx\adapters\openai\headers_test.go
 * @Description: 认证头缓存测试 —— 命中复用/轮换失效/空 key/并发安全，
 * 对应 headers() 共享只读缓存优化
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// sameMap 比较两个 map 是否同一实例（底层指针一致）.
// [EN] Whether two maps are the same instance.
func sameMap(a, b map[string]string) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

// TestHeaders_CacheReuse 验证密钥未变时返回同一 map（零分配复用）.
// [EN] Verify the same map is returned while the key is unchanged.
func TestHeaders_CacheReuse(t *testing.T) {
	c := New("sk-1")
	h1 := c.headers()
	h2 := c.headers()
	assert.True(t, sameMap(h1, h2)) // 共享实例
	assert.Equal(t, "Bearer sk-1", h1["Authorization"])
}

// TestHeaders_RotationInvalidates 验证密钥轮换后缓存失效重建.
// [EN] Verify rotation invalidates the cache.
func TestHeaders_RotationInvalidates(t *testing.T) {
	c := New("sk-1")
	h1 := c.headers()
	c.SetAPIKey("sk-2")
	h2 := c.headers()
	assert.False(t, sameMap(h1, h2))
	assert.Equal(t, "Bearer sk-2", h2["Authorization"])
	assert.Equal(t, "Bearer sk-1", h1["Authorization"]) // 旧实例不受影响
}

// TestHeaders_EmptyKey 验证空密钥无认证头.
// [EN] Verify empty keys carry no auth header.
func TestHeaders_EmptyKey(t *testing.T) {
	c := New("")
	h := c.headers()
	assert.NotContains(t, h, "Authorization")
	assert.Empty(t, h)
}

// TestHeaders_ConcurrentAccess 验证并发请求下缓存读写安全.
// [EN] Verify concurrent access is safe.
func TestHeaders_ConcurrentAccess(t *testing.T) {
	c := New("sk-1")
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h := c.headers()
			assert.Equal(t, "Bearer sk-1", h["Authorization"])
		}()
	}
	wg.Wait()
}
