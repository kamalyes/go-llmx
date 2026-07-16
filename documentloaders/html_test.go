/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 20:33:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 20:33:58
 * @FilePath: \go-llmx\documentloaders\html_test.go
 * @Description: HTML 加载器测试 —— 噪声标签跳过/块级换行/空正文，
 * 纯字符串输入零依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documentloaders

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTML_ExtractsText 验证正文提取与噪声跳过.
// [EN] Verify text extraction and noise skipping.
func TestHTML_ExtractsText(t *testing.T) {
	doc := `<!DOCTYPE html>
<html><head><title>Page</title><style>body{color:red}</style></head>
<body>
<h1>Hello</h1>
<p>First paragraph.</p>
<p>Second paragraph.</p>
<script>alert(1)</script>
</body></html>`

	docs, err := NewHTML(strings.NewReader(doc)).WithSource("page.html").Load(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 1)

	text := docs[0].PageContent
	assert.Contains(t, text, "Hello")
	assert.Contains(t, text, "First paragraph.")
	assert.Contains(t, text, "Second paragraph.")
	assert.NotContains(t, text, "alert(1)")  // script 跳过
	assert.NotContains(t, text, "color:red") // style 跳过
	assert.Equal(t, "page.html", docs[0].Metadata["source"])
}

// TestHTML_BlockNewlines 验证块级元素换行结构.
// [EN] Verify block-element newlines.
func TestHTML_BlockNewlines(t *testing.T) {
	docs, err := NewHTML(strings.NewReader(`<p>a</p><p>b</p>`)).Load(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 1)
	lines := strings.Split(docs[0].PageContent, "\n")
	assert.Equal(t, []string{"a", "", "b"}, lines) // 块间空行
}

// TestHTML_EmptyBody 验证无正文返回空.
// [EN] Verify an empty body yields nothing.
func TestHTML_EmptyBody(t *testing.T) {
	docs, err := NewHTML(strings.NewReader(`<html><head><title>t</title></head><body></body></html>`)).Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, docs) // head 跳过后无正文
}
