/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 16:05:47
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 16:05:47
 * @FilePath: \go-llmx\chain\conversational_retrieval_qa.go
 * @Description: 多轮检索问答链 —— 历史改写 + 检索 + 作答 + 记忆回写，
 * 替代 langchaingo ConversationalRetrievalQA
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/memory"
	"github.com/kamalyes/go-llmx/prompt"
)

// condenseQuestionPrompt 历史改写模板（对话问题 → 独立检索问题）.
// [EN] Condense template (dialogue question → standalone retrieval query).
const condenseQuestionPrompt = `Given the following conversation and a follow up question,
rewrite the follow up question to be a standalone question, in its original language.

Chat History:
{{.History}}

Follow Up Question: {{.Input}}
Standalone Question:`

// answerWithContextPrompt 带上下文作答模板.
// [EN] Answer-with-context template.
const answerWithContextPrompt = `Answer the question based only on the following context:

{{.Context}}

Question: {{.Input}}

Helpful Answer:`

// condenseVars 改写模板变量.
// [EN] Condense template variables.
type condenseVars struct {
	// History 对话历史文本.
	// [EN] Conversation history text.
	History string

	// Input 本轮问题.
	// [EN] Follow-up question.
	Input string
}

// ConversationalRetrievalQA 多轮检索问答链：本轮问题先经改写模型
// 消解指代（"它/他"→具体实体）成为独立检索问题，再检索作答；
// 无历史时跳过改写直接检索（省一次 LLM 调用）.
// [EN] Multi-turn retrieval QA: condense the follow-up into a standalone
// question before retrieval; skip condensing when no history exists.
type ConversationalRetrievalQA struct {
	// Model 作答模型.
	// [EN] Answering model.
	Model llmx.Model

	// CondenseModel 改写模型（nil 复用 Model）.
	// [EN] Condensing model (reuses Model when nil).
	CondenseModel llmx.Model

	// Retriever 检索器.
	// [EN] Retriever.
	Retriever llmx.Retriever

	// Memory 会话记忆（nil 无状态单轮）.
	// [EN] Conversation memory (nil = stateless).
	Memory memory.Memory

	// condenseTpl 改写模板（nil 走默认）.
	// [EN] Condense template.
	condenseTpl *prompt.Template

	// answerTpl 作答模板（nil 走默认）.
	// [EN] Answer template.
	answerTpl *prompt.Template
}

// NewConversationalRetrievalQA 构造多轮检索问答链.
// [EN] Build a conversational retrieval-QA chain.
func NewConversationalRetrievalQA(model llmx.Model, retriever llmx.Retriever) *ConversationalRetrievalQA {
	return &ConversationalRetrievalQA{
		Model:     model,
		Retriever: retriever,
	}
}

// WithCondenseModel 注入独立改写模型（可用轻量模型省钱，nil 复用主模型）.
// [EN] Inject a dedicated condense model (lightweight to save cost).
func (c *ConversationalRetrievalQA) WithCondenseModel(m llmx.Model) *ConversationalRetrievalQA {
	c.CondenseModel = m
	return c
}

// WithMemory 注入会话记忆（多轮指代消解依赖历史）.
// [EN] Inject conversation memory.
func (c *ConversationalRetrievalQA) WithMemory(m memory.Memory) *ConversationalRetrievalQA {
	c.Memory = m
	return c
}

// WithCondensePrompt 覆盖改写模板（变量 {{.History}} 与 {{.Input}}）.
// [EN] Override the condense template.
func (c *ConversationalRetrievalQA) WithCondensePrompt(tpl *prompt.Template) *ConversationalRetrievalQA {
	c.condenseTpl = tpl
	return c
}

// WithAnswerPrompt 覆盖作答模板（变量 {{.Context}} 与 {{.Input}}）.
// [EN] Override the answer template.
func (c *ConversationalRetrievalQA) WithAnswerPrompt(tpl *prompt.Template) *ConversationalRetrievalQA {
	c.answerTpl = tpl
	return c
}

// Run 多轮问答：改写 → 检索 → 作答 → 记忆回写.
// [EN] Run: condense → retrieve → answer → memorize.
func (c *ConversationalRetrievalQA) Run(ctx context.Context, input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("%w: conversational QA requires a question", llmx.ErrInvalidRequest)
	}
	if c.Model == nil || c.Retriever == nil {
		return "", fmt.Errorf("%w: conversational QA requires a model and a retriever", llmx.ErrInvalidRequest)
	}

	var history []llmx.Message
	if c.Memory != nil {
		history = c.Memory.Messages()
	}

	// 首轮无历史：问题本就独立，跳过改写省一次 LLM 调用
	// [EN] First turn: question is already standalone; skip condensing.
	question := input
	if len(history) > 0 {
		condensed, err := c.condense(ctx, history, input)
		if err != nil {
			return "", err
		}
		question = condensed
	}

	docs, err := c.Retriever.GetRelevantDocuments(ctx, question)
	if err != nil {
		return "", fmt.Errorf("retrieve: %w", err)
	}
	if len(docs) == 0 {
		return "I don't have enough context to answer this question.", nil
	}

	rendered, err := renderDocumentPrompt(c.answerTpl, answerWithContextPrompt, documentVars{
		Context: joinDocuments(docs),
		Input:   input, // 作答用原始问题（用户措辞），检索才用改写问题
	})
	if err != nil {
		return "", err
	}

	messages := history
	messages = append(messages, llmx.User(rendered))
	resp, err := c.Model.GenerateContent(ctx, messages)
	if err != nil {
		return "", err
	}
	answer, err := llmx.FirstText(resp)
	if err != nil {
		return "", err
	}

	if c.Memory != nil {
		c.Memory.Add(llmx.User(input), llmx.Assistant(answer))
	}
	return answer, nil
}

// condense 历史改写：对话问题 → 独立检索问题.
// [EN] Condense: dialogue question → standalone retrieval question.
func (c *ConversationalRetrievalQA) condense(ctx context.Context, history []llmx.Message, input string) (string, error) {
	model := c.CondenseModel
	if model == nil {
		model = c.Model
	}
	tpl := c.condenseTpl
	if tpl == nil {
		compiled, err := prompt.New(condenseQuestionPrompt)
		if err != nil {
			return "", err
		}
		tpl = compiled
	}
	rendered, err := tpl.Render(condenseVars{
		History: joinMessages(history),
		Input:   input,
	})
	if err != nil {
		return "", err
	}
	resp, err := model.GenerateContent(ctx, []llmx.Message{llmx.User(rendered)})
	if err != nil {
		return "", err
	}
	return llmx.FirstText(resp)
}

// joinMessages 拼接历史为文本块（角色: 内容 形态，紧凑无空行）.
// [EN] Join history into a compact text block.
func joinMessages(msgs []llmx.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.String())
		b.WriteByte('\n')
	}
	return b.String()
}
