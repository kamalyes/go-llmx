/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 21:15:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 21:15:00
 * @FilePath: \go-llmx\chain\chain_test.go
 * @Description: 链式编排契约测试 —— 接口实现断言
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package chain

import "testing"

func TestChainInterfaceCompliance(t *testing.T) {
	// 编译期断言三种链均实现 Chain
	var _ Chain = (*LLMChain)(nil)
	var _ Chain = (*ConversationChain)(nil)
	var _ Chain = (*SequentialChain)(nil)
}
