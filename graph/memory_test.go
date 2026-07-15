/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-15 20:51:32
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-15 21:08:59
 * @FilePath: \go-llmx\graph\memory_test.go
 * @Description: 图谱记忆单测 —— 观测入图/实体召回（一跳全量 +
 * 二跳截断）/RecallText 拼接/清空，注入式 fake 提取器零 LLM 依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package graph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeExtractor 固定应答提取器（记录观测文本）.
// [EN] A fixed-reply extractor (records observed texts).
type fakeExtractor struct {
	// reply 应答三元组.
	// [EN] Replied triples.
	reply []Triple

	// observed 观测过的文本.
	// [EN] Observed texts.
	observed []string

	// err 注入错误.
	// [EN] Injected error.
	err error
}

// Extract 实现 TripleExtractor.
// [EN] Implement TripleExtractor.
func (f *fakeExtractor) Extract(_ context.Context, text string) ([]Triple, error) {
	f.observed = append(f.observed, text)
	if f.err != nil {
		return nil, f.err
	}
	return f.reply, nil
}

// newMemoryWithSeed 构造图谱记忆并注入种子三元组.
// [EN] Build graph memory seeded with triples.
func newMemoryWithSeed(triples ...Triple) (*GraphMemory, *fakeExtractor) {
	f := &fakeExtractor{reply: triples}
	return NewGraphMemory(f), f
}

// TestRemember 验证观测入图与新增计数.
// [EN] Verify observing into the graph and add counting.
func TestRemember(t *testing.T) {
	m, f := newMemoryWithSeed(
		Triple{Subject: "小明", Predicate: "工作于", Object: "鹅厂"},
	)
	n, err := m.Remember(context.Background(), "小明在鹅厂上班")
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, f.observed, 1)
	assert.Equal(t, "小明在鹅厂上班", f.observed[0])
	assert.Equal(t, 1, m.Graph().Size())

	// 重复观测去重，新增 0
	n, err = m.Remember(context.Background(), "再说一遍")
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// TestRemember_Empty 验证空文本跳过提取.
// [EN] Verify blank text skips extraction.
func TestRemember_Empty(t *testing.T) {
	m, f := newMemoryWithSeed()
	n, err := m.Remember(context.Background(), "  ")
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, f.observed)
}

// TestRemember_Error 验证提取错误透传.
// [EN] Verify extractor error propagation.
func TestRemember_Error(t *testing.T) {
	f := &fakeExtractor{err: errBoom}
	m := NewGraphMemory(f)
	_, err := m.Remember(context.Background(), "text")
	assert.ErrorIs(t, err, errBoom)
}

// TestRecall 验证实体召回（一跳全量 + 二跳扩散 + 排序确定性）.
// [EN] Verify recall (one-hop full + two-hop spread + determinism).
func TestRecall(t *testing.T) {
	m, _ := newMemoryWithSeed(
		Triple{Subject: "小明", Predicate: "工作于", Object: "鹅厂"},
		Triple{Subject: "小明", Predicate: "喜欢", Object: "篮球"},
		Triple{Subject: "鹅厂", Predicate: "位于", Object: "深圳"},
		Triple{Subject: "篮球", Predicate: "属于", Object: "运动"},
	)
	_, err := m.Remember(context.Background(), "seed")
	require.NoError(t, err)

	triples, err := m.Recall(context.Background(), "介绍一下小明的工作")
	require.NoError(t, err)
	require.Len(t, triples, 4) // 一跳 2 + 二跳（鹅厂/篮球出边各 1）
	assert.Contains(t, triples, Triple{Subject: "鹅厂", Predicate: "位于", Object: "深圳"})

	// 无关查询空召回
	triples, err = m.Recall(context.Background(), "今天天气怎么样")
	require.NoError(t, err)
	assert.Empty(t, triples)
}

// TestRecall_CaseInsensitive 验证实体命中忽略大小写.
// [EN] Verify case-insensitive entity matching.
func TestRecall_CaseInsensitive(t *testing.T) {
	m, _ := newMemoryWithSeed(
		Triple{Subject: "Go", Predicate: "releases", Object: "1.25"},
	)
	_, err := m.Remember(context.Background(), "seed")
	require.NoError(t, err)
	triples, err := m.Recall(context.Background(), "tell me about go")
	require.NoError(t, err)
	require.Len(t, triples, 1)
}

// TestRecall_Hop2Limit 验证二跳截断上限.
// [EN] Verify the two-hop truncation cap.
func TestRecall_Hop2Limit(t *testing.T) {
	m, _ := newMemoryWithSeed()
	var seed []Triple
	seed = append(seed, Triple{Subject: "root", Predicate: "r", Object: "hub"})
	for i := 0; i < recallHop2Limit; i++ {
		seed = append(seed, Triple{
			Subject: "hub", Predicate: "leaf", Object: "leaf" + string(rune('a'+i%26))})
	}
	m.Graph().Add(seed...)

	triples, err := m.Recall(context.Background(), "root")
	require.NoError(t, err)
	assert.LessOrEqual(t, len(triples), recallHop2Limit+1)
}

// TestRecallText 验证召回文本拼接（prompt 注入形态）.
// [EN] Verify recall text joining (prompt-ready shape).
func TestRecallText(t *testing.T) {
	m, _ := newMemoryWithSeed(
		Triple{Subject: "小明", Predicate: "工作于", Object: "鹅厂"},
	)
	_, err := m.Remember(context.Background(), "seed")
	require.NoError(t, err)
	text, err := m.RecallText(context.Background(), "小明在哪工作")
	require.NoError(t, err)
	assert.Equal(t, "小明 工作于 鹅厂.", text)

	text, err = m.RecallText(context.Background(), "无关查询")
	require.NoError(t, err)
	assert.Equal(t, "", text)
}

// TestClear 验证清空.
// [EN] Verify clearing.
func TestClear(t *testing.T) {
	m, _ := newMemoryWithSeed(
		Triple{Subject: "a", Predicate: "p", Object: "b"},
	)
	_, err := m.Remember(context.Background(), "seed")
	require.NoError(t, err)
	require.Equal(t, 1, m.Graph().Size())
	m.Clear()
	assert.Equal(t, 0, m.Graph().Size())
	assert.Empty(t, m.Graph().Nodes())
}
