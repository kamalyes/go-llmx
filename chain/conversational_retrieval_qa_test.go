/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-16 16:31:52
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-16 16:31:52
 * @FilePath: \go-llmx\chain\conversational_retrieval_qa_test.go
 * @Description: 多轮检索问答链测试 —— 指代改写/首轮跳过改写/记忆回写/
 * 独立改写模型/检索空兜底，FakeModel 注入零网络依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"strings"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRetriever 固定文档检索器（记录检索问题）.
// [EN] Fixed-doc retriever (records queries).
type fakeRetriever struct {
	// docs 固定返回文档.
	// [EN] Canned documents.
	docs []llmx.Document

	// queries 检索过的问题（验证改写结果）.
	// [EN] Queries seen (to verify condensing).
	queries []string
}

// GetRelevantDocuments 实现 Retriever.
// [EN] Implement Retriever.
func (r *fakeRetriever) GetRelevantDocuments(_ context.Context, query string) ([]llmx.Document, error) {
	r.queries = append(r.queries, query)
	return r.docs, nil
}

// lastQuery 最近一次检索问题.
// [EN] The most recent query.
func (r *fakeRetriever) lastQuery() string {
	if len(r.queries) == 0 {
		return ""
	}
	return r.queries[len(r.queries)-1]
}

// condenseFake 按序应答的改写模型（首个调用改写、次个作答语义）.
// [EN] Sequential-reply model (first call condenses).
type condenseFake struct {
	// replies 按序返回的文本.
	// [EN] Sequential replies.
	replies []string

	// prompts 收到的提示词.
	// [EN] Prompts received.
	prompts []string
}

// GenerateContent 实现 Model.
// [EN] Implement Model.
func (m *condenseFake) GenerateContent(_ context.Context, msgs []llmx.Message, _ ...llmx.Option) (*llmx.Response, error) {
	reply := ""
	if len(m.replies) > 0 {
		reply = m.replies[0]
		m.replies = m.replies[1:]
	}
	m.prompts = append(m.prompts, msgs[0].String())
	return &llmx.Response{
		Choices: []llmx.Choice{{Content: []llmx.Part{llmx.TextPart{Text: reply}}}},
	}, nil
}

// StreamGenerateContent 实现 Model（测试不触达）.
// [EN] Implement Model (unused in these tests).
func (m *condenseFake) StreamGenerateContent(_ context.Context, _ []llmx.Message, _ llmx.StreamHandler, _ ...llmx.Option) (*llmx.Response, error) {
	return m.GenerateContent(context.Background(), nil)
}

// TestConversationalQA_FirstTurnSkipsCondense 验证首轮无历史直接检索.
// [EN] Verify the first turn skips condensing.
func TestConversationalQA_FirstTurnSkipsCondense(t *testing.T) {
	ret := &fakeRetriever{docs: []llmx.Document{{PageContent: "Redis is a cache."}}}
	m := &condenseFake{replies: []string{"Redis is a distributed cache."}}
	c := NewConversationalRetrievalQA(m, ret).WithMemory(memory.NewBuffer())

	answer, err := c.Run(context.Background(), "what is Redis?")
	require.NoError(t, err)
	assert.Equal(t, "Redis is a distributed cache.", answer)

	// 无历史不改写：检索问题即原始输入，LLM 只被调用一次（作答）
	// [EN] No history: the retrieval query equals the input; one LLM call.
	assert.Equal(t, "what is Redis?", ret.lastQuery())
	assert.Len(t, m.prompts, 1)
}

// TestConversationalQA_CondensesWithHistory 验证次轮经改写消解指代.
// [EN] Verify the follow-up turn is condensed.
func TestConversationalQA_CondensesWithHistory(t *testing.T) {
	ret := &fakeRetriever{docs: []llmx.Document{{PageContent: "Go is fast."}}}
	m := &condenseFake{replies: []string{
		"first answer",                    // 首轮作答
		"What is Go's concurrency model?", // 次轮改写
		"Go uses goroutines.",             // 次轮作答
	}}
	c := NewConversationalRetrievalQA(m, ret).WithMemory(memory.NewBuffer())

	// 首轮建立历史
	// [EN] First turn builds history.
	_, err := c.Run(context.Background(), "what is Go?")
	require.NoError(t, err)

	// 次轮指代问题：检索用的是改写后的问题
	// [EN] Second turn: retrieval uses the condensed query.
	answer, err := c.Run(context.Background(), "how does its concurrency work?")
	require.NoError(t, err)
	assert.Equal(t, "Go uses goroutines.", answer)
	assert.Equal(t, "What is Go's concurrency model?", ret.lastQuery())

	// 改写提示词含历史与新问题
	// [EN] The condense prompt carries history and the follow-up.
	require.Len(t, m.prompts, 3)
	assert.Contains(t, m.prompts[1], "how does its concurrency work?")
	assert.Contains(t, m.prompts[1], "user: what is Go?")
}

// TestConversationalQA_MemorizesTurns 验证记忆回写本轮问答对.
// [EN] Verify the turn is memorized.
func TestConversationalQA_MemorizesTurns(t *testing.T) {
	ret := &fakeRetriever{docs: []llmx.Document{{PageContent: "ctx"}}}
	buf := memory.NewBuffer()
	m := &condenseFake{replies: []string{"ans1", "condensed q2", "ans2"}}
	c := NewConversationalRetrievalQA(m, ret).WithMemory(buf)

	_, err := c.Run(context.Background(), "q1")
	require.NoError(t, err)
	_, err = c.Run(context.Background(), "q2")
	require.NoError(t, err)

	msgs := buf.Messages()
	require.Len(t, msgs, 4) // 两轮 user/assistant 对
	assert.Equal(t, "q1", msgs[0].String())
	assert.Equal(t, "ans1", msgs[1].String())
	assert.Equal(t, "q2", msgs[2].String())
	assert.Equal(t, "ans2", msgs[3].String())
}

// TestConversationalQA_DedicatedCondenseModel 验证独立改写模型路由.
// [EN] Verify routing to a dedicated condense model.
func TestConversationalQA_DedicatedCondenseModel(t *testing.T) {
	ret := &fakeRetriever{docs: []llmx.Document{{PageContent: "ctx"}}}
	answerer := &condenseFake{replies: []string{"final answer"}}
	condenser := &condenseFake{replies: []string{"standalone question"}}
	buf := memory.NewBuffer()
	c := NewConversationalRetrievalQA(answerer, ret).
		WithCondenseModel(condenser).
		WithMemory(buf)

	// 首轮（作答走主模型）
	// [EN] First turn (answered by the main model).
	_, err := c.Run(context.Background(), "first")
	require.NoError(t, err)
	// 次轮：改写走 condenser、作答走 answerer
	// [EN] Second turn: condense via condenser, answer via answerer.
	_, err = c.Run(context.Background(), "its details")
	require.NoError(t, err)

	assert.Empty(t, condenser.prompts[:0]) // 首轮未触达
	require.Len(t, condenser.prompts, 1)   // 次轮改写一次
	require.Len(t, answerer.prompts, 2)    // 两轮各作答一次
}

// TestConversationalQA_EmptyRetrieval 验证检索空兜底话术.
// [EN] Verify the empty-retrieval fallback.
func TestConversationalQA_EmptyRetrieval(t *testing.T) {
	ret := &fakeRetriever{}
	m := &condenseFake{}
	c := NewConversationalRetrievalQA(m, ret)

	answer, err := c.Run(context.Background(), "anything")
	require.NoError(t, err)
	assert.Equal(t, "I don't have enough context to answer this question.", answer)
}

// TestConversationalQA_EmptyQuestion 验证空问题报错.
// [EN] Verify blank question errors.
func TestConversationalQA_EmptyQuestion(t *testing.T) {
	c := NewConversationalRetrievalQA(&condenseFake{}, &fakeRetriever{})
	_, err := c.Run(context.Background(), "  ")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// TestConversationalQA_NilDependencies 验证缺模型/检索器报错.
// [EN] Verify missing model/retriever errors.
func TestConversationalQA_NilDependencies(t *testing.T) {
	c := NewConversationalRetrievalQA(&condenseFake{}, &fakeRetriever{})
	c.Model = nil
	_, err := c.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)

	c2 := NewConversationalRetrievalQA(&condenseFake{}, &fakeRetriever{})
	c2.Retriever = nil
	_, err = c2.Run(context.Background(), "q")
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// TestJoinMessages 验证历史拼接紧凑形态.
// [EN] Verify compact history joining.
func TestJoinMessages(t *testing.T) {
	msgs := []llmx.Message{llmx.User("hi"), llmx.Assistant("hello")}
	out := joinMessages(msgs)
	assert.True(t, strings.HasPrefix(out, "user: hi\n"))
	assert.True(t, strings.HasSuffix(out, "assistant: hello\n"))
}
