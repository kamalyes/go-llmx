/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-17 22:27:36
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-17 22:35:16
 * @FilePath: \go-llmx\outputparser\boolean.go
 * @Description: 布尔输出解析器 —— YES/NO/TRUE/FALSE 归一（大小写/引号/反引号容错）.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"fmt"
	"strings"
)

// Boolean 布尔解析器（自定义真/假词表）.
// [EN] Boolean parser (customizable true/false vocabularies).
type Boolean struct {
	// trueStrings 判定为真的词表（归一后精确匹配）.
	// [EN] Words that parse as true (matched after normalization).
	trueStrings []string

	// falseStrings 判定为假的词表.
	// [EN] Words that parse as false.
	falseStrings []string
}

// NewBoolean 构造布尔解析器（默认 YES/TRUE 与 NO/FALSE）.
// [EN] Build a boolean parser (defaults YES/TRUE and NO/FALSE).
func NewBoolean() *Boolean {
	return &Boolean{
		trueStrings:  []string{"YES", "TRUE"},
		falseStrings: []string{"NO", "FALSE"},
	}
}

// WithVocabulary 覆盖真/假词表（大小写不敏感；用于中文场景如 是/否）.
// [EN] Override true/false vocabularies (case-insensitive).
func (p *Boolean) WithVocabulary(trueWords, falseWords []string) *Boolean {
	p.trueStrings = normalizeAll(trueWords)
	p.falseStrings = normalizeAll(falseWords)
	return p
}

// Parse 实现 Parser[bool].
// [EN] Implement Parser[bool].
func (p *Boolean) Parse(text string) (bool, error) {
	normalized := normalizeText(text)
	for _, w := range p.trueStrings {
		if normalized == w {
			return true, nil
		}
	}
	for _, w := range p.falseStrings {
		if normalized == w {
			return false, nil
		}
	}
	return false, ParseError{
		Text:   text,
		Reason: fmt.Sprintf("expected one of %v, got %q", append(p.trueStrings, p.falseStrings...), normalized),
	}
}

// GetFormatInstructions 实现 Parser[bool].
// [EN] Implement Parser[bool].
func (p *Boolean) GetFormatInstructions() string {
	return "Your output should be a single boolean word, e.g. `true` or `false`."
}

// normalizeText 归一（去空白、去引号/反引号、转大写）.
// [EN] Normalize (trim, strip quotes, uppercase).
func normalizeText(text string) string {
	return strings.ToUpper(strings.Trim(strings.TrimSpace(text), "'\"`"))
}

// normalizeAll 词表归一（构造期一次性完成，Parse 热路径零分配）.
// [EN] Normalize a vocabulary (done once at construction).
func normalizeAll(words []string) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = normalizeText(w)
	}
	return out
}

// 编译期断言：实现 Parser 契约.
// [EN] Compile-time contract assertion.
var _ Parser[bool] = (*Boolean)(nil)
