/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 21:59:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 21:59:00
 * @FilePath: \go-llmx\memory\memory.go
 * @Description: 会话记忆契约 —— Memory 接口，
 * 实现见 buffer.go / window.go，重型后端由适配器扩展
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package memory

import llmx "github.com/kamalyes/go-llmx"

// Memory 会话记忆（对话历史的存取抽象）.
// [EN] Conversation memory (history storage abstraction).
type Memory interface {
	// Messages 返回当前历史（返回副本，调用方修改不影响内部状态）.
	// [EN] Return the current history (a copy).
	Messages() []llmx.Message

	// Add 追加消息.
	// [EN] Append messages.
	Add(messages ...llmx.Message)

	// Clear 清空历史.
	// [EN] Clear the history.
	Clear()
}
