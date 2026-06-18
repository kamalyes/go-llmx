/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-18 21:18:55
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-18 21:26:08
 * @FilePath: \go-llmx\outputparser\list.go
 * @Description: 逗号分隔列表解析器 —— "foo, bar, baz" → []string（逐项去空白）.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import "strings"

// List 逗号分隔列表解析器.
// [EN] Comma-separated list parser.
type List struct{}

// NewList 构造列表解析器.
// [EN] Build a list parser.
func NewList() *List { return &List{} }

// Parse 实现 Parser[[]string]（空串解析为空列表）.
// [EN] Implement Parser[[]string] (empty text yields an empty list).
func (p *List) Parse(text string) ([]string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []string{}, nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, len(parts))
	for i, part := range parts {
		out[i] = strings.TrimSpace(part)
	}
	return out, nil
}

// GetFormatInstructions 实现 Parser[[]string].
// [EN] Implement Parser[[]string].
func (p *List) GetFormatInstructions() string {
	return "Your response should be a list of comma separated values, e.g. `foo, bar, baz`."
}

// 编译期断言：实现 Parser 契约.
// [EN] Compile-time contract assertion.
var _ Parser[[]string] = (*List)(nil)
