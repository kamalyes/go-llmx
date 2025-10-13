/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 22:26:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 22:26:00
 * @FilePath: \go-llmx\memory\memory_test.go
 * @Description: 会话记忆契约测试 —— 接口实现断言 + 并发安全
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
)

func TestMemoryInterfaceCompliance(t *testing.T) {
	// 编译期断言两种实现均满足 Memory 契约
	var _ Memory = (*Buffer)(nil)
	var _ Memory = (*Window)(nil)
}

func TestMemoryConcurrent(t *testing.T) {
	m := NewBuffer()
	w := NewWindow(10)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Add(llmx.User("x"))
			w.Add(llmx.User("x"))
			_ = m.Messages()
			_ = w.Messages()
		}()
	}
	wg.Wait()
	assert.Len(t, m.Messages(), 50)
	assert.Len(t, w.Messages(), 10)
}
