/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-25 21:21:18
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-25 21:28:12
 * @FilePath: \go-llmx\chain\retrievalqa_test.go
 * @Description: 检索问答链测试 —— 检索注入/空结果兜底/模板覆盖/参数校验.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/prompt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listRetriever 固定返回预设文档的检索器.
// [EN] Retriever returning canned documents.
type listRetriever struct {
	docs []llmx.Document
	err  error
}

func (r listRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]llmx.Document, error) {
	return r.docs, r.err
}

func retrievalQAFixture() (*promptCapture, *RetrievalQA) {
	cap := &promptCapture{}
	r := listRetriever{docs: []llmx.Document{
		{PageContent: "Go 1.25 was released in August 2025."},
		{PageContent: "Go has generics since 1.18."},
	}}
	return cap, NewRetrievalQA(cap, r)
}

func TestRetrievalQA_ContextInjected(t *testing.T) {
	cap, qa := retrievalQAFixture()

	answer, err := qa.Run(context.Background(), "When was Go 1.25 released?")
	require.NoError(t, err)
	assert.Equal(t, "answer-1", answer)

	require.Len(t, cap.prompts, 1)
	assert.Contains(t, cap.prompts[0], "Go 1.25 was released in August 2025.")
	assert.Contains(t, cap.prompts[0], "When was Go 1.25 released?")
}

func TestRetrievalQA_EmptyRetrievalHonestFallback(t *testing.T) {
	cap := &promptCapture{}
	qa := NewRetrievalQA(cap, listRetriever{})

	answer, err := qa.Run(context.Background(), "anything")
	require.NoError(t, err)
	assert.Contains(t, answer, "don't have enough context")
	assert.Empty(t, cap.prompts)
}

func TestRetrievalQA_RetrieverError(t *testing.T) {
	_, qa := retrievalQAFixture()
	qa.Retriever = listRetriever{err: assert.AnError}

	_, err := qa.Run(context.Background(), "q")
	require.ErrorIs(t, err, assert.AnError)
}

func TestRetrievalQA_Validation(t *testing.T) {
	_, qa := retrievalQAFixture()

	_, err := qa.Run(context.Background(), "  ")
	require.ErrorIs(t, err, llmx.ErrInvalidRequest)

	// 缺模型
	qa2 := NewRetrievalQA(nil, listRetriever{docs: docFixture()})
	_, err = qa2.Run(context.Background(), "q")
	require.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

func TestRetrievalQA_CustomPrompt(t *testing.T) {
	cap, qa := retrievalQAFixture()
	tpl, err := prompt.New("C: {{.Context}}\nQ: {{.Input}}")
	require.NoError(t, err)
	qa.WithPrompt(tpl)

	_, err = qa.Run(context.Background(), "q")
	require.NoError(t, err)
	require.Len(t, cap.prompts, 1)
	assert.Contains(t, cap.prompts[0], "C: Go 1.25")
}
