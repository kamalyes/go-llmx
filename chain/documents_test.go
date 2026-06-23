/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-23 21:12:38
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-23 21:19:03
 * @FilePath: \go-llmx\chain\documents_test.go
 * @Description: 文档处理链测试 —— 三种编排形态的调用次数/模板渲染/失败路径
 * （FakeModel 捕获 prompt 断言内容形态）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"
	"strings"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/prompt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// promptCapture 捕获每次调用的 prompt 并按轮次返回固定文本.
// [EN] Captures prompts and returns canned text per round.
type promptCapture struct {
	prompts []string
	round   int
}

func (p *promptCapture) GenerateContent(ctx context.Context, messages []llmx.Message, opts ...llmx.Option) (*llmx.Response, error) {
	p.prompts = append(p.prompts, messages[0].String())
	p.round++
	return &llmx.Response{
		Choices: []llmx.Choice{{
			Content: []llmx.Part{llmx.TextPart{Text: fmt.Sprintf("answer-%d", p.round)}},
		}},
	}, nil
}

func (p *promptCapture) StreamGenerateContent(ctx context.Context, messages []llmx.Message, handler llmx.StreamHandler, opts ...llmx.Option) (*llmx.Response, error) {
	return p.GenerateContent(ctx, messages, opts...)
}

func docFixture() []llmx.Document {
	return []llmx.Document{
		{PageContent: "Paris is the capital of France."},
		{PageContent: "The Seine river flows through Paris."},
		{PageContent: "France is in Western Europe."},
	}
}

func TestStuff_SingleCall(t *testing.T) {
	cap := &promptCapture{}
	c := NewStuff(cap)

	answer, err := c.Run(context.Background(), docFixture(), "What is the capital of France?")
	require.NoError(t, err)
	assert.Equal(t, "answer-1", answer)

	// 单次调用，prompt 含全部文档与查询
	require.Len(t, cap.prompts, 1)
	assert.Contains(t, cap.prompts[0], "Paris is the capital of France.")
	assert.Contains(t, cap.prompts[0], "The Seine river flows through Paris.")
	assert.Contains(t, cap.prompts[0], "What is the capital of France?")
}

func TestMapReduce_1PlusNCalls(t *testing.T) {
	cap := &promptCapture{}
	c := NewMapReduce(cap)
	docs := docFixture()

	answer, err := c.Run(context.Background(), docs, "Where is France?")
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("answer-%d", len(docs)+1), answer)

	// map 阶段 N 次 + reduce 阶段 1 次
	require.Len(t, cap.prompts, len(docs)+1)
	// 每次调用 map prompt 只含单个文档
	assert.Contains(t, cap.prompts[0], "Paris is the capital of France.")
	assert.NotContains(t, cap.prompts[0], "The Seine river")
	// reduce 阶段合并全部摘要与查询
	assert.Contains(t, cap.prompts[3], "answer-1")
	assert.Contains(t, cap.prompts[3], "answer-2")
	assert.Contains(t, cap.prompts[3], "Where is France?")
}

func TestRefine_NSequentialCalls(t *testing.T) {
	cap := &promptCapture{}
	c := NewRefine(cap)
	docs := docFixture()

	answer, err := c.Run(context.Background(), docs, "Describe Paris.")
	require.NoError(t, err)
	assert.Equal(t, "answer-3", answer)

	// N 文档 N 次顺序调用
	require.Len(t, cap.prompts, len(docs))
	// 首轮无 Answer，第二轮起携带前一轮答案
	assert.NotContains(t, cap.prompts[0], "We have provided an existing answer: answer-")
	assert.Contains(t, cap.prompts[1], "answer-1")
	assert.Contains(t, cap.prompts[2], "answer-2")
	// 每轮携带查询
	for _, p := range cap.prompts {
		assert.Contains(t, p, "Describe Paris.")
	}
}

func TestDocumentChain_Validation(t *testing.T) {
	for name, c := range map[string]DocumentChain{
		"stuff":     NewStuff(&promptCapture{}),
		"mapreduce": NewMapReduce(&promptCapture{}),
		"refine":    NewRefine(&promptCapture{}),
	} {
		_, err := c.Run(context.Background(), nil, "q")
		require.ErrorIs(t, err, llmx.ErrInvalidRequest, name)

		_, err = c.Run(context.Background(), docFixture(), "  ")
		require.ErrorIs(t, err, llmx.ErrInvalidRequest, name)
	}
}

func TestStuff_CustomPrompt(t *testing.T) {
	cap := &promptCapture{}
	tpl, err := prompt.New("CTX: {{.Context}}\nQ: {{.Input}}")
	require.NoError(t, err)
	c := NewStuff(cap).WithPrompt(tpl)

	_, err = c.Run(context.Background(), docFixture()[:1], "q")
	require.NoError(t, err)
	require.Len(t, cap.prompts, 1)
	assert.True(t, strings.HasPrefix(cap.prompts[0], "CTX: Paris is the capital of France."))
}
