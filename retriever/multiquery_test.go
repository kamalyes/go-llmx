/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-17 22:38:57
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-17 22:38:57
 * @FilePath: \go-llmx\retriever\multiquery_test.go
 * @Description: 多查询检索器测试 —— 变体扩展与渲染/并行召回有序去重/
 * 空输出退化单路/错误传播/自定义提示词/参数校验.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package retriever

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingRetriever 记录查询并按规则回放的检索器.
// [EN] Retriever recording queries and replaying by rule.
type recordingRetriever struct {
	mu      sync.Mutex
	queries []string
	respond func(q string) ([]llmx.Document, error)
}

func (r *recordingRetriever) GetRelevantDocuments(_ context.Context, q string) ([]llmx.Document, error) {
	r.mu.Lock()
	r.queries = append(r.queries, q)
	r.mu.Unlock()
	return r.respond(q)
}

// collected 读取已记录查询的排序快照（并行写入下的稳定断言）.
// [EN] Sorted snapshot of recorded queries (stable under parallel writes).
func (r *recordingRetriever) collected() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.queries...)
	sort.Strings(out)
	return out
}

func TestMultiQuery_VariantsRetrieveAndDedupInOrder(t *testing.T) {
	fm := llmx.NewFakeModel("V1\n\nV2\nV3")
	ret := &recordingRetriever{respond: func(q string) ([]llmx.Document, error) {
		switch q {
		case "原始问题", "V1":
			return []llmx.Document{{PageContent: "docA"}, {PageContent: "docB"}}, nil
		case "V2":
			return []llmx.Document{{PageContent: "docB"}, {PageContent: "docC"}}, nil
		default:
			return []llmx.Document{{PageContent: "docD"}}, nil
		}
	}}

	docs, err := NewMultiQuery(ret, fm).GetRelevantDocuments(context.Background(), "原始问题")
	require.NoError(t, err)

	// 模型单次调用；四路查询（原查询 + 三变体）
	require.Equal(t, 1, fm.CallCount())
	assert.Equal(t, []string{"V1", "V2", "V3", "原始问题"}, ret.collected())

	// 按路序去重：原始路 docA/docB → V1 全重 → V2 增 docC → V3 增 docD
	require.Len(t, docs, 4)
	assert.Equal(t, []string{"docA", "docB", "docC", "docD"}, []string{docs[0].PageContent, docs[1].PageContent, docs[2].PageContent, docs[3].PageContent})
}

func TestMultiQuery_PromptRendersOriginalQuestion(t *testing.T) {
	fm := llmx.NewFakeModel("V1")
	ret := &recordingRetriever{respond: func(string) ([]llmx.Document, error) { return nil, nil }}

	_, err := NewMultiQuery(ret, fm).GetRelevantDocuments(context.Background(), "什么是嵌入")
	require.NoError(t, err)

	// 默认提示词渲染出原始查询
	require.Equal(t, 1, fm.CallCount())
	assert.Contains(t, fm.LastCall()[0].String(), "什么是嵌入")
}

func TestMultiQuery_CustomPrompt(t *testing.T) {
	fm := llmx.NewFakeModel("V1")
	ret := &recordingRetriever{respond: func(string) ([]llmx.Document, error) { return nil, nil }}
	r := NewMultiQuery(ret, fm)
	r.Prompt = "Rewrite for retrieval: {{.question}}"

	_, err := r.GetRelevantDocuments(context.Background(), "q")
	require.NoError(t, err)
	assert.Contains(t, fm.LastCall()[0].String(), "Rewrite for retrieval: q")
}

func TestMultiQuery_EmptyVariantsFallback(t *testing.T) {
	fm := llmx.NewFakeModel("  \n \n")
	ret := &recordingRetriever{respond: func(string) ([]llmx.Document, error) {
		return []llmx.Document{{PageContent: "docA"}}, nil
	}}

	docs, err := NewMultiQuery(ret, fm).GetRelevantDocuments(context.Background(), "q")
	require.NoError(t, err)

	// 无有效变体 → 仅原始查询单路
	require.Len(t, docs, 1)
	assert.Equal(t, []string{"q"}, ret.collected())
}

func TestMultiQuery_ModelErrorPropagates(t *testing.T) {
	fm := llmx.NewFakeModel()
	fm.Err = errors.New("boom")
	ret := &recordingRetriever{respond: func(string) ([]llmx.Document, error) { return nil, nil }}

	_, err := NewMultiQuery(ret, fm).GetRelevantDocuments(context.Background(), "q")
	assert.ErrorContains(t, err, "boom")
}

func TestMultiQuery_VariantRetrievalErrorPropagates(t *testing.T) {
	fm := llmx.NewFakeModel("V1")
	ret := &recordingRetriever{respond: func(q string) ([]llmx.Document, error) {
		if q == "V1" {
			return nil, errors.New("variant down")
		}
		return nil, nil
	}}

	_, err := NewMultiQuery(ret, fm).GetRelevantDocuments(context.Background(), "q")
	assert.ErrorContains(t, err, "variant down")
}

func TestMultiQuery_NilDeps(t *testing.T) {
	r := &MultiQueryRetriever{}
	_, err := r.GetRelevantDocuments(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}
