/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-20 13:59:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-20 15:02:31
 * @FilePath: \go-llmx\outputparser\combining.go
 * @Description: 组合解析器 —— 按序尝试多个解析器，首个成功者胜出.
 * 适合模型输出形态不稳定的场景（先试 JSON，退化为裸文本兜底）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"errors"
	"fmt"
)

// Combining 组合解析器（同一结果类型 T 的多解析器按序回退）.
// [EN] Combining parser (ordered fallback over parsers of the same type T).
type Combining[T any] struct {
	// parsers 按序尝试的解析器列表.
	// [EN] Parsers tried in order.
	parsers []Parser[T]
}

// NewCombining 构造组合解析器（至少一个解析器）.
// [EN] Build a combining parser (at least one required).
func NewCombining[T any](parsers ...Parser[T]) (*Combining[T], error) {
	if len(parsers) == 0 {
		return nil, fmt.Errorf("%w: combining parser requires at least one parser", ErrParse)
	}
	return &Combining[T]{parsers: parsers}, nil
}

// MustNewCombining 构造组合解析器（空列表 panic，用于包级变量初始化）.
// [EN] Build a combining parser (panics when empty).
func MustNewCombining[T any](parsers ...Parser[T]) *Combining[T] {
	c, err := NewCombining(parsers...)
	if err != nil {
		panic(err)
	}
	return c
}

// Parse 实现 Parser[T]（首个成功者胜出；全部失败时聚合各解析器原因）.
// [EN] Implement Parser[T] (first success wins; all failures aggregated).
func (p *Combining[T]) Parse(text string) (T, error) {
	var zero T
	var reasons []string
	for _, parser := range p.parsers {
		if parser == nil {
			continue
		}
		v, err := parser.Parse(text)
		if err == nil {
			return v, nil
		}
		if errors.Is(err, ErrParse) {
			reasons = append(reasons, err.Error())
			continue
		}
		// 非 ParseError 的意外错误直接上抛（如 JSON 反序列化内部错误）
		return zero, err
	}
	if len(reasons) == 0 {
		return zero, ParseError{Text: text, Reason: "no usable parser in combining chain"}
	}
	return zero, ParseError{Text: text, Reason: "all parsers failed: " + joinReasons(reasons)}
}

// GetFormatInstructions 实现 Parser[T]（取首个解析器的指令，它是主输出形态）.
// [EN] Implement Parser[T] (first parser's instructions, the primary format).
func (p *Combining[T]) GetFormatInstructions() string {
	for _, parser := range p.parsers {
		if parser != nil {
			return parser.GetFormatInstructions()
		}
	}
	return ""
}

// joinReasons 聚合失败原因（截断防爆炸）.
// [EN] Aggregate failure reasons (truncated).
func joinReasons(reasons []string) string {
	out := ""
	for i, r := range reasons {
		if i > 0 {
			out += "; "
		}
		out += r
	}
	return out
}

// 编译期断言：实现 Parser 契约.
// [EN] Compile-time contract assertion.
var _ Parser[map[string]any] = (*Combining[map[string]any])(nil)
