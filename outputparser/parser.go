/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-17 21:02:03
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-17 21:08:37
 * @FilePath: \go-llmx\outputparser\parser.go
 * @Description: 输出解析器契约 —— Parser[T] 泛型接口与 ParseError.
 * LLM 文本输出 → 类型化结果；GetFormatInstructions 生成提示词格式指令
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"errors"
	"fmt"
)

// ErrParse 解析失败基类哨兵（ParseError 实现了本错误，errors.Is 可判定）.
// [EN] Sentinel base for parse failures (ParseError implements it).
var ErrParse = errors.New("outputparser: parse failed")

// ParseError 解析失败详情（携带原文与原因，errors.Is(err, ErrParse) 判定）.
// [EN] Parse failure detail (carries source text and reason).
type ParseError struct {
	// Text 解析失败的原始输出.
	// [EN] The raw output that failed to parse.
	Text string

	// Reason 失败原因.
	// [EN] Failure reason.
	Reason string
}

// Error 实现 error.
// [EN] Implement error.
func (e ParseError) Error() string {
	return fmt.Sprintf("outputparser: parse text %q: %s", e.Text, e.Reason)
}

// Unwrap 对接哨兵（errors.Is(err, ErrParse) 成立）.
// [EN] Unwrap to the sentinel.
func (e ParseError) Unwrap() error { return ErrParse }

// Parser[T] 输出解析器（LLM 文本 → 类型化结果）.
// [EN] Output parser (LLM text to a typed result).
type Parser[T any] interface {
	// Parse 解析模型输出文本.
	// [EN] Parse the model output text.
	Parse(text string) (T, error)

	// GetFormatInstructions 拼入提示词的输出格式指令（告知模型应输出什么形态）.
	// [EN] Format instructions to embed in prompts.
	GetFormatInstructions() string
}
