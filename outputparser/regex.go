/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-18 22:39:55
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-18 22:51:29
 * @FilePath: \go-llmx\outputparser\regex.go
 * @Description: 正则捕获组解析器 —— 命名捕获组 → map（组名即 key）.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"fmt"
	"regexp"
)

// Regex 正则解析器（命名捕获组驱动）.
// [EN] Regex parser (driven by named capture groups).
type Regex struct {
	// expression 编译后的正则.
	// [EN] Compiled expression.
	expression *regexp.Regexp

	// outputKeys 命名捕获组列表（与子匹配一一对应）.
	// [EN] Named capture groups (aligned with submatches).
	outputKeys []string
}

// NewRegex 构造正则解析器（表达式须含命名捕获组，如 `(?P<answer>.+)`）.
// [EN] Build a regex parser (expression must contain named groups).
func NewRegex(expression string) (*Regex, error) {
	re, err := regexp.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid regex %q: %v", ErrParse, expression, err)
	}
	return &Regex{expression: re, outputKeys: re.SubexpNames()[1:]}, nil
}

// MustNewRegex 构造正则解析器（非法表达式 panic，用于包级变量初始化）.
// [EN] Build a regex parser (panics on invalid expressions).
func MustNewRegex(expression string) *Regex {
	p, err := NewRegex(expression)
	if err != nil {
		panic(err)
	}
	return p
}

// Parse 实现 Parser[map[string]string]（首个整体匹配的命名组抽取）.
// [EN] Implement Parser[map[string]string].
func (p *Regex) Parse(text string) (map[string]string, error) {
	match := p.expression.FindStringSubmatch(text)
	if match == nil {
		return nil, ParseError{
			Text:   text,
			Reason: fmt.Sprintf("no match for expression %q", p.expression.String()),
		}
	}
	// match[0] 为整体匹配，子匹配从 1 起，与 outputKeys 对齐
	out := make(map[string]string, len(p.outputKeys))
	for i, key := range p.outputKeys {
		if key != "" && i+1 < len(match) {
			out[key] = match[i+1]
		}
	}
	return out, nil
}

// GetFormatInstructions 实现 Parser[map[string]string].
// [EN] Implement Parser[map[string]string].
func (p *Regex) GetFormatInstructions() string {
	return "Your output should conform to the expected pattern."
}

// 编译期断言：实现 Parser 契约.
// [EN] Compile-time contract assertion.
var _ Parser[map[string]string] = (*Regex)(nil)
