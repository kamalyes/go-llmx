/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 20:51:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 20:51:00
 * @FilePath: \go-llmx\textsplitter\textsplitter_test.go
 * @Description: 递归分块器测试 —— 分隔符降级/重叠衔接/硬切兜底/文档封装
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package textsplitter

import (
	"strings"
	"testing"
	"unicode/utf8"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplit_Empty(t *testing.T) {
	s := NewRecursiveSplitter()
	assert.Nil(t, s.Split(""))
}

func TestSplit_ShortTextSingleChunk(t *testing.T) {
	s := NewRecursiveSplitter()
	chunks := s.Split("短文本")
	assert.Equal(t, []string{"短文本"}, chunks)
}

func TestSplit_ByParagraph(t *testing.T) {
	s := NewRecursiveSplitter(WithChunkSize(12), WithChunkOverlap(0))
	text := "第一段落内容。\n\n第二段落内容。"
	chunks := s.Split(text)

	// 两段合并超限 → 保留段落边界分块
	require.Len(t, chunks, 2)
	assert.Equal(t, "第一段落内容。\n\n", chunks[0])
	assert.Equal(t, "第二段落内容。", chunks[1])
}

func TestSplit_RecursiveFallback(t *testing.T) {
	// chunkSize 小于段落 → 降级到 \n / 句读继续拆
	s := NewRecursiveSplitter(WithChunkSize(6), WithChunkOverlap(0))
	text := "第一段落。\n\n第二段落。"
	chunks := s.Split(text)

	for _, c := range chunks {
		assert.LessOrEqual(t, utf8.RuneCountInString(c), 6)
	}
	// 内容无损拼接还原（忽略重复空片）
	joined := strings.Join(chunks, "")
	assert.Equal(t, text, joined)
}

func TestSplit_HardCutFallback(t *testing.T) {
	// 无任何分隔符命中的长串 → 字符级兜底
	s := NewRecursiveSplitter(WithChunkSize(4), WithChunkOverlap(0), WithSeparators("", "\n"))
	// hmm 分隔符只有 \n 与 ""，长串不含 \n → 硬切
	text := "abcdefghijklmnop"
	chunks := s.Split(text)

	require.Greater(t, len(chunks), 1)
	for _, c := range chunks {
		assert.LessOrEqual(t, utf8.RuneCountInString(c), 4)
	}
	assert.Equal(t, text, strings.Join(chunks, ""))
}

func TestSplit_OverlapChaining(t *testing.T) {
	s := NewRecursiveSplitter(WithChunkSize(8), WithChunkOverlap(3), WithSeparators("", " "))
	// 仅空格与字符级分隔符：句子按词拆后合并，块间尾部 3 runes 重叠
	text := "aa bb cc dd ee ff gg hh ii jj"
	chunks := s.Split(text)

	require.Greater(t, len(chunks), 1)
	for i := 1; i < len(chunks); i++ {
		prev, next := chunks[i-1], chunks[i]
		// 下一块以上一块尾部（去除可能的空格起始）作为衔接
		tail := runeTail(prev, 3)
		assert.True(t, strings.HasPrefix(next, tail) || strings.HasPrefix(next, strings.TrimLeft(tail, " ")),
			"块 %d 应与上一块重叠衔接: prev=%q next=%q", i, prev, next)
	}
}

func TestSplit_CJK(t *testing.T) {
	s := NewRecursiveSplitter(WithChunkSize(10), WithChunkOverlap(0))
	text := strings.Repeat("这是中文句子。", 5)
	chunks := s.Split(text)

	require.Greater(t, len(chunks), 1)
	var total int
	for _, c := range chunks {
		n := utf8.RuneCountInString(c)
		assert.LessOrEqual(t, n, 10)
		total += n
	}
	assert.Equal(t, utf8.RuneCountInString(text), total)
}

func TestSplitter_OptionsValidation(t *testing.T) {
	// 非法 chunkSize 回落默认
	s := NewRecursiveSplitter(WithChunkSize(0))
	assert.Equal(t, DefaultChunkSize, s.chunkSize)

	// overlap >= chunkSize 时禁用重叠；负值归零
	s2 := NewRecursiveSplitter(WithChunkSize(10), WithChunkOverlap(99))
	assert.Equal(t, 10, s2.chunkSize)
	assert.Equal(t, 0, s2.chunkOverlap)
	s3 := NewRecursiveSplitter(WithChunkOverlap(-5))
	assert.Equal(t, 0, s3.chunkOverlap)

	// 空分隔符回落默认
	s4 := NewRecursiveSplitter(WithSeparators())
	assert.Equal(t, DefaultSeparators, s4.separators)

	// nil 选项安全跳过
	s5 := NewRecursiveSplitter(nil)
	assert.Equal(t, DefaultChunkSize, s5.chunkSize)
}

func TestRuneTail(t *testing.T) {
	assert.Equal(t, "", runeTail("abcd", 0))
	assert.Equal(t, "cd", runeTail("abcd", 2))
	// n 超长返回原串
	assert.Equal(t, "abcd", runeTail("abcd", 10))
	// rune 边界（中文按字符而非字节截取）
	assert.Equal(t, "文", runeTail("中文", 1))
}

func TestSplitDocuments(t *testing.T) {
	s := NewRecursiveSplitter(WithChunkSize(5), WithChunkOverlap(0), WithSeparators("", " "))
	meta := map[string]any{"src": "测试"}
	docs := SplitDocuments(s, "aa bb cc dd", meta)

	require.Greater(t, len(docs), 1)
	for _, d := range docs {
		assert.Equal(t, "测试", d.Metadata["src"])
		assert.NotEmpty(t, d.PageContent)
	}
}

func TestSplitterInterface(t *testing.T) {
	var _ Splitter = (*RecursiveSplitter)(nil)
	var _ = llmx.Document{} // 核心类型引用编译期确认
}
