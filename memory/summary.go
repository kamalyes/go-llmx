/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-16 18:02:11
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-16 18:02:11
 * @FilePath: \go-llmx\memory\summary.go
 * @Description: 摘要记忆 —— LLM 压缩历史轮次为滚动摘要 +
 * 保留最近 K 轮原文，长对话上下文可控且早期信息不丢
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	llmx "github.com/kamalyes/go-llmx"
)

// summaryPrompt 摘要合并模板（旧摘要 + 待压缩对话 → 新摘要）.
// [EN] Summary merge template (old summary + turns → new summary).
const summaryPrompt = `Progressively summarize the lines of conversation by adding on the top of the existing summary, in the original language.

Existing Summary:
%s

New Lines of Conversation:
%s

New Summary:`

// summaryTurn 轮（user/assistant 成对）.
// [EN] A turn (user/assistant pair).
type summaryTurn struct {
	userText      string
	assistantText string
}

// Summary 摘要记忆：滚动摘要 + 最近 keepPairs 轮原文.
// [EN] Summary memory: rolling summary + recent turns.
type Summary struct {
	mu sync.Mutex

	// model 压缩用 LLM.
	// [EN] Condensing LLM.
	model llmx.Model

	// keepPairs 保留原文的最近轮数（<=0 兜底 2）.
	// [EN] Recent turns kept verbatim.
	keepPairs int

	// summaryText 当前滚动摘要（空串表示还没有摘要）.
	// [EN] The rolling summary ("" = none yet).
	summaryText string

	// turns 未压缩的轮队列.
	// [EN] Uncompressed turn queue.
	turns []summaryTurn
}

// NewSummary 构造摘要记忆（keepPairs<=0 时兜底 2）.
// [EN] Build a summary memory (keepPairs clamped to 2).
func NewSummary(model llmx.Model, keepPairs int) *Summary {
	if keepPairs <= 0 {
		keepPairs = 2
	}
	return &Summary{model: model, keepPairs: keepPairs}
}

// Add 实现 Memory（按 user/assistant 对入队；奇数条等待配对）.
// [EN] Implement Memory (pairs queue up; odd ones wait).
func (s *Summary) Add(messages ...llmx.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range messages {
		switch m.Role {
		case llmx.RoleUser:
			s.turns = append(s.turns, summaryTurn{userText: m.String()})
		case llmx.RoleAssistant:
			if n := len(s.turns); n > 0 && s.turns[n-1].assistantText == "" {
				s.turns[n-1].assistantText = m.String()
				continue
			}
			s.turns = append(s.turns, summaryTurn{assistantText: m.String()})
		}
	}
}

// Messages 实现 Memory：摘要（如有）+ 全部未压缩轮原文
// （不做隐式 LLM 调用；压缩走显式 Condense）.
// [EN] Implement Memory: summary (if any) + verbatim turns.
func (s *Summary) Messages() []llmx.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []llmx.Message
	if s.summaryText != "" {
		out = append(out, llmx.System("Summary of the conversation so far: "+s.summaryText))
	}
	for _, t := range s.turns {
		if t.userText != "" {
			out = append(out, llmx.User(t.userText))
		}
		if t.assistantText != "" {
			out = append(out, llmx.Assistant(t.assistantText))
		}
	}
	return out
}

// Condense 显式压缩：超出 keepPairs 的最老轮合并进摘要（幂等，可重入）.
// [EN] Explicitly condense: merge turns beyond keepPairs into the summary.
func (s *Summary) Condense(ctx context.Context) error {
	if s.model == nil {
		return fmt.Errorf("%w: summary memory requires a model to condense", llmx.ErrInvalidRequest)
	}

	s.mu.Lock()
	if len(s.turns) <= s.keepPairs {
		s.mu.Unlock()
		return nil
	}
	merged := s.turns[:len(s.turns)-s.keepPairs]
	s.turns = append([]summaryTurn(nil), s.turns[len(s.turns)-s.keepPairs:]...)
	old := s.summaryText
	s.mu.Unlock()

	rendered := fmt.Sprintf(summaryPrompt, orEmpty(old), joinTurns(merged))
	resp, err := s.model.GenerateContent(ctx, []llmx.Message{llmx.User(rendered)})
	if err != nil {
		// 压缩失败：轮次回滚，原文不丢
		// [EN] Failed condense: roll back, keep verbatim turns.
		s.mu.Lock()
		s.turns = append(merged, s.turns...)
		s.mu.Unlock()
		return err
	}
	text, err := llmx.FirstText(resp)
	if err != nil {
		s.mu.Lock()
		s.turns = append(merged, s.turns...)
		s.mu.Unlock()
		return err
	}

	s.mu.Lock()
	s.summaryText = strings.TrimSpace(text)
	s.mu.Unlock()
	return nil
}

// Clear 实现 Memory（摘要与轮队列清空）.
// [EN] Implement Memory (resets summary and turns).
func (s *Summary) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaryText = ""
	s.turns = nil
}

// joinTurns 轮队列拼接为文本块（紧凑）.
// [EN] Join turns into a compact block.
func joinTurns(turns []summaryTurn) string {
	var b strings.Builder
	for _, t := range turns {
		if t.userText != "" {
			b.WriteString("user: ")
			b.WriteString(t.userText)
			b.WriteByte('\n')
		}
		if t.assistantText != "" {
			b.WriteString("assistant: ")
			b.WriteString(t.assistantText)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// orEmpty 空串替代（摘要缺省话术）.
// [EN] Empty-string default.
func orEmpty(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
