/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-19 22:17:39
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-19 22:33:05
 * @FilePath: \go-llmx\outputparser\structured.go
 * @Description: 结构化输出解析器 —— ```json 代码块提取 → map（必填字段校验），
 * GetFormatInstructions 按 schema 生成格式指令
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Field 结构化字段描述（名称 + 期望内容说明）.
// [EN] Structured field descriptor (name + expected content).
type Field struct {
	// Name 字段名（解析结果 map 的 key）.
	// [EN] Field name (the key in the parsed map).
	Name string

	// Description 字段内容说明（拼入格式指令告知模型）.
	// [EN] Content description (embedded in format instructions).
	Description string

	// Type 字段类型提示（拼入格式指令；解析结果仍为宽松 JSON 值）.
	// [EN] Type hint (instructions only; parsed values stay loose JSON).
	Type string
}

// Structured 结构化解析器（JSON 代码块 + 必填字段校验）.
// [EN] Structured parser (JSON code block + required-field validation).
type Structured struct {
	// fields 字段描述列表.
	// [EN] Field descriptors.
	fields []Field
}

// NewStructured 构造结构化解析器.
// [EN] Build a structured parser.
func NewStructured(fields ...Field) *Structured {
	return &Structured{fields: fields}
}

// Parse 实现 Parser[map[string]any]（```json 块提取 → 反序列化 → 必填校验）.
// [EN] Implement Parser[map[string]any].
func (p *Structured) Parse(text string) (map[string]any, error) {
	_, after, ok := strings.Cut(text, "```json")
	if !ok {
		return nil, ParseError{Text: text, Reason: "no ```json block in output"}
	}
	jsonText, _, ok := strings.Cut(after, "```")
	if !ok {
		return nil, ParseError{Text: text, Reason: "unterminated ```json block"}
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonText)), &parsed); err != nil {
		return nil, ParseError{Text: text, Reason: "invalid JSON: " + err.Error()}
	}
	// 必填字段校验
	var missing []string
	for _, f := range p.fields {
		if _, ok := parsed[f.Name]; !ok {
			missing = append(missing, f.Name)
		}
	}
	if len(missing) > 0 {
		return nil, ParseError{
			Text:   text,
			Reason: fmt.Sprintf("missing required fields %v", missing),
		}
	}
	return parsed, nil
}

// GetFormatInstructions 实现 Parser[map[string]any]（按字段表生成 ```json 模板）.
// [EN] Implement Parser[map[string]any].
func (p *Structured) GetFormatInstructions() string {
	var b strings.Builder
	b.WriteString("The output should be a markdown code snippet formatted in the following schema:\n```json\n{\n")
	for _, f := range p.fields {
		typ := f.Type
		if typ == "" {
			typ = "string"
		}
		fmt.Fprintf(&b, "\t%q: %s // %s\n", f.Name, typ, f.Description)
	}
	b.WriteString("}\n```")
	return b.String()
}

// 编译期断言：实现 Parser 契约.
// [EN] Compile-time contract assertion.
var _ Parser[map[string]any] = (*Structured)(nil)
