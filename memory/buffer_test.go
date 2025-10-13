/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 22:35:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 22:35:00
 * @FilePath: \go-llmx\memory\buffer_test.go
 * @Description: 全量缓冲记忆测试 —— 存取/副本语义
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
)

func TestBuffer(t *testing.T) {
	m := NewBuffer()
	assert.Empty(t, m.Messages())

	m.Add(llmx.User("a"), llmx.Assistant("b"))
	m.Add(llmx.System("s"))
	msgs := m.Messages()
	assert.Len(t, msgs, 3)
	assert.Equal(t, "a", msgs[0].Content[0].(llmx.TextPart).Text)

	// 副本语义：外部修改不影响内部
	msgs[0] = llmx.User("tampered")
	assert.Equal(t, "a", m.Messages()[0].Content[0].(llmx.TextPart).Text)

	m.Clear()
	assert.Empty(t, m.Messages())
}
