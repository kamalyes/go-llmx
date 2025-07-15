/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-15 21:07:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-15 21:07:00
 * @FilePath: \go-llmx\llm.go
 * @Description: LLM 模型核心抽象 —— Model 接口（GenerateContent + 流式）.
 * 全部适配器实现本接口；无 Deprecated 方法、无六层 callbacks（StreamHandler 单回调覆盖）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"context"
	"errors"
)

// Model 多模态对话模型接口（全部适配器的统一契约）.
// [EN] Multimodal chat model interface (the universal contract of all adapters).
type Model interface {
	// 生成对话内容.
	// [EN] Generate content from chat messages.
	GenerateContent(ctx context.Context, messages []Message, opts ...Option) (*Response, error)

	// 流式生成对话内容.
	// [EN] Generate content with streaming callback per chunk.
	//
	// stream 为 nil 时退化为一次性返回（等价 GenerateContent）；
	// handler 返回 ErrStopStream 提前终止，此时已收内容仍会随 Response 返回，
	// 并将 error 置为 ErrStreamClosed
	StreamGenerateContent(ctx context.Context, messages []Message, stream StreamHandler, opts ...Option) (*Response, error)
}

// Generate 单提示词便捷调用（非流式，返回首个候选的文本）.
// [EN] Convenience call with a single prompt, returns first choice text.
func Generate(ctx context.Context, m Model, prompt string, opts ...Option) (string, error) {
	resp, err := m.GenerateContent(ctx, []Message{User(prompt)}, opts...)
	if err != nil {
		return "", err
	}
	return FirstText(resp)
}

// FirstText 提取首个候选的文本，无候选报 ErrEmptyResponse.
// [EN] Extract text of the first choice, ErrEmptyResponse if none.
func FirstText(resp *Response) (string, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return "", ErrEmptyResponse
	}
	return resp.Choices[0].Text(), nil
}

// reasonErr 包装用户回调错误为流终止信号（适配器内部使用）.
// [EN] Wrap handler error as stream termination signal (adapter-internal use).
func reasonErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrStopStream) {
		return ErrStreamClosed
	}
	return err
}
