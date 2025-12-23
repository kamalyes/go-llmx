/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-09 23:07:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-23 21:52:00
 * @FilePath: \go-llmx\fake.go
 * @Description: 内存 Fake 模型 —— 无网络单测 / 框架联调桩.
 * 预设回复按序消费，同时捕获请求供断言（替代 langchaingo testcontainers 的轻量方案）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"context"
	"fmt"
	"sync"
)

// FakeModel 预设回复的内存模型（测试与本地联调）.
// [EN] In-memory model with canned responses (for tests and local wiring).
type FakeModel struct {
	mu sync.Mutex

	// Responses 预设回复序列：每次调用按序弹出，耗尽后复读最后一个.
	Responses []*Response

	// StreamTexts 预设流式文本序列：StreamGenerateContent 按字符流式吐出.
	StreamTexts []string

	// Err 非空时所有调用直接返回该错误.
	Err error

	// StreamErr 非 nil 且当前调用为流式时，吐完文本后返回该错误.
	StreamErr error

	// 捕获的调用记录（GenerateRequests[i] 对应第 i 次调用的输入）
	Calls        [][]Message
	CallOptions  []*Options
	streamHandle func(chunk *Chunk) error
}

// NewFakeModel 构造 FakeModel 并注入预设回复.
// [EN] Build a FakeModel with canned responses.
func NewFakeModel(texts ...string) *FakeModel {
	f := &FakeModel{}
	for _, t := range texts {
		f.Responses = append(f.Responses, &Response{
			Choices: []Choice{{Content: []Part{TextPart{Text: t}}, FinishReason: "stop"}},
			Model:   "fake",
		})
	}
	return f
}

// GenerateContent 实现非流式生成（按序弹出预设回复）.
// [EN] Implement non-streaming generation (pop canned response in order).
func (f *FakeModel) GenerateContent(_ context.Context, messages []Message, opts ...Option) (*Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, messages)
	f.CallOptions = append(f.CallOptions, Apply(opts...))
	if f.Err != nil {
		return nil, f.Err
	}
	if len(f.Responses) == 0 {
		return nil, ErrEmptyResponse
	}
	resp := f.Responses[0]
	if len(f.Responses) > 1 {
		f.Responses = f.Responses[1:]
	}
	return resp, nil
}

// StreamGenerateContent 实现流式生成（将预设文本按字符切帧吐出）.
// [EN] Implement streaming generation (emit canned text char by char).
func (f *FakeModel) StreamGenerateContent(_ context.Context, messages []Message, stream StreamHandler, opts ...Option) (*Response, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, messages)
	f.CallOptions = append(f.CallOptions, Apply(opts...))
	if f.Err != nil {
		f.mu.Unlock()
		return nil, f.Err
	}
	text := ""
	if len(f.Responses) > 0 {
		text = f.Responses[0].Choices[0].Text()
		if len(f.Responses) > 1 {
			f.Responses = f.Responses[1:]
		}
	}
	f.mu.Unlock()

	var collected string
	if stream != nil {
		for _, r := range text {
			ch := &Chunk{Content: string(r)}
			err := stream(ch)
			// 当前帧已投递给回调，无论回调是否终止都计入已收集内容
			collected += string(r)
			if err != nil {
				return assembleStreamed(collected, f), reasonErr(err)
			}
		}
		if f.StreamErr != nil {
			return assembleStreamed(collected, f), f.StreamErr
		}
		if err := stream(&Chunk{FinishReason: "stop"}); err != nil {
			return assembleStreamed(collected, f), reasonErr(err)
		}
	}
	return assembleStreamed(text, f), nil
}

// assembleStreamed 将流式收集文本封装为完整响应.
// [EN] Wrap collected stream text into a full response.
func assembleStreamed(text string, f *FakeModel) *Response {
	return &Response{
		Choices: []Choice{{Content: []Part{TextPart{Text: text}}, FinishReason: "stop"}},
		Model:   "fake",
	}
}

// LastCall 返回最近一次调用的消息输入（无调用报错提示）.
// [EN] Return the messages of the most recent call.
func (f *FakeModel) LastCall() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Calls) == 0 {
		return nil
	}
	return f.Calls[len(f.Calls)-1]
}

// CallCount 返回累计调用次数.
// [EN] Return the total call count.
func (f *FakeModel) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Calls)
}

// String 实现 fmt.Stringer（调试辅助）.
// [EN] Implement fmt.Stringer for debugging.
func (f *FakeModel) String() string {
	return fmt.Sprintf("FakeModel{calls=%d, responses=%d}", f.CallCount(), len(f.Responses))
}
