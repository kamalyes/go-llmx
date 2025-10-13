/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 20:19:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 20:19:00
 * @FilePath: \go-llmx\chain\chain.go
 * @Description: 链式编排契约 —— Chain 接口 + 模板渲染数据载体，
 * 实现见 llmchain.go / conversation.go / sequential.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package chain

import "context"

// Chain 链式执行抽象（单输入单输出的编排单元）.
// [EN] Chain abstraction (single-input single-output orchestration unit).
type Chain interface {
	// Run 以输入文本执行链路，返回输出文本.
	// [EN] Run the chain with an input string, returning the output.
	Run(ctx context.Context, input string) (string, error)
}

// chainInput 模板渲染数据载体（模板中以 {{.Input}} 引用输入）.
// [EN] Data carrier for template rendering ({{.Input}} in templates).
type chainInput struct {
	Input string
}
