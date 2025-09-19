/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-19 22:15:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-19 22:15:00
 * @FilePath: \go-llmx\prompt\template.go
 * @Description: 提示词模板 —— 基于 Go 标准库 text/template（零依赖），
 * 砍掉 langchaingo 的 sprig / gonja 模板引擎依赖
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package prompt

import (
	"bytes"
	"fmt"
	"text/template"

	llmx "github.com/kamalyes/go-llmx"
)

// Template 提示词模板（Go text/template 语法）.
// [EN] Prompt template (Go text/template syntax).
type Template struct {
	// tmpl 编译后的模板.
	// [EN] Compiled template.
	tmpl *template.Template
}

// New 编译模板文本.
// [EN] Compile template text.
func New(text string) (*Template, error) {
	tmpl, err := template.New("prompt").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", llmx.ErrInvalidRequest, err)
	}
	return &Template{tmpl: tmpl}, nil
}

// Render 以 data 渲染模板（data 为模板变量载体，如 map / struct）.
// [EN] Render the template with data (a map or struct).
func (p *Template) Render(data any) (string, error) {
	var buf bytes.Buffer
	if err := p.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("%w: %v", llmx.ErrInvalidRequest, err)
	}
	return buf.String(), nil
}
