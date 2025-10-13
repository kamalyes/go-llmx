/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 22:39:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 22:39:00
 * @FilePath: \go-llmx\memory\window_test.go
 * @Description: 滑动窗口记忆测试 —— 滑窗保留/非法 k 兜底
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
)

func TestWindow_Sliding(t *testing.T) {
	m := NewWindow(3)
	m.Add(llmx.User("1"), llmx.Assistant("2"), llmx.User("3"), llmx.Assistant("4"))

	msgs := m.Messages()
	assert.Len(t, msgs, 3)
	// 保留最近 3 条：2、3、4
	assert.Equal(t, "2", msgs[0].Content[0].(llmx.TextPart).Text)
	assert.Equal(t, "4", msgs[2].Content[0].(llmx.TextPart).Text)

	m.Clear()
	assert.Empty(t, m.Messages())
}

func TestWindow_NonPositiveK(t *testing.T) {
	// k<=0 兜底为 1
	m := NewWindow(0)
	m.Add(llmx.User("a"), llmx.User("b"))
	msgs := m.Messages()
	assert.Len(t, msgs, 1)
	assert.Equal(t, "b", msgs[0].Content[0].(llmx.TextPart).Text)
}
