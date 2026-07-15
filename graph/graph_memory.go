/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-15 20:51:32
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-15 21:03:47
 * @FilePath: \go-llmx\graph\graph_memory.go
 * @Description: 图谱记忆 —— 文本观测经提取器入图，查询按命中实体
 * 召回相关三元组（一跳全量 + 二跳截断），RecallText 直接可注入 prompt
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package graph

import (
	"context"
	"sort"
	"strings"
)

// recallHop2Limit 二跳召回三元组截断（防大图全量回吐）.
// [EN] Two-hop recall truncation (prevents dumping large graphs).
const recallHop2Limit = 32

// GraphMemory 图谱记忆（观测入图 + 实体召回）.
// [EN] Graph memory (observe into graph + entity recall).
type GraphMemory struct {
	// graph 底层图谱.
	// [EN] Underlying graph.
	graph *Graph

	// extractor 三元组提取器（LLM 或注入实现）.
	// [EN] Triple extractor (LLM or injected).
	extractor TripleExtractor
}

// NewGraphMemory 构造图谱记忆.
// [EN] Build graph memory.
func NewGraphMemory(extractor TripleExtractor) *GraphMemory {
	return &GraphMemory{
		graph:     NewGraph(),
		extractor: extractor,
	}
}

// Graph 暴露底层图谱（直查与图算法）.
// [EN] Expose the underlying graph (direct queries and algorithms).
func (m *GraphMemory) Graph() *Graph {
	return m.graph
}

// Remember 观测文本：提取三元组入图，返回实际新增数量.
// [EN] Observe text: extract triples into the graph; returns the added count.
func (m *GraphMemory) Remember(ctx context.Context, text string) (int, error) {
	if strings.TrimSpace(text) == "" {
		return 0, nil
	}
	triples, err := m.extractor.Extract(ctx, text)
	if err != nil {
		return 0, err
	}
	return m.graph.Add(triples...), nil
}

// Recall 查询召回：从查询文本提取实体，返回图中相关三元组
// （命中实体一跳全量 + 邻居二跳截断，排序确定性）.
// [EN] Recall for a query: extract entities, return related triples
// (one-hop full + two-hop truncated, deterministically sorted).
func (m *GraphMemory) Recall(ctx context.Context, query string) ([]Triple, error) {
	entities, err := m.entities(ctx, query)
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	var related []Triple
	collect := func(ts []Triple) {
		for _, t := range ts {
			k := t.key()
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			related = append(related, t)
		}
	}

	for _, e := range entities {
		collect(m.graph.TriplesBySubject(e))
		collect(m.graph.TriplesByObject(e))
	}
	if len(related) >= recallHop2Limit {
		return sortedTriples(related), nil
	}

	hop2 := map[string]struct{}{}
	for _, e := range entities {
		for _, n := range m.graph.Neighbors(e) {
			hop2[n] = struct{}{}
		}
	}
	for n := range hop2 {
		collect(m.graph.TriplesBySubject(n))
		if len(related) >= recallHop2Limit {
			break
		}
	}
	return sortedTriples(related), nil
}

// entities 从查询文本提取候选实体（图谱全部实体的子串命中，忽略大小写）.
// [EN] Candidate entities from query text (substring match on graph nodes, case-insensitive).
func (m *GraphMemory) entities(_ context.Context, query string) ([]string, error) {
	q := strings.ToLower(query)
	var hit []string
	for _, node := range m.graph.Nodes() {
		if node != "" && strings.Contains(q, strings.ToLower(node)) {
			hit = append(hit, node)
		}
	}
	return hit, nil
}

// RecallText 召回并拼接为文本（空召回返回空串，直接注入 prompt）.
// [EN] Recall and join as text (empty string when nothing hits).
func (m *GraphMemory) RecallText(ctx context.Context, query string) (string, error) {
	triples, err := m.Recall(ctx, query)
	if err != nil {
		return "", err
	}
	if len(triples) == 0 {
		return "", nil
	}
	lines := make([]string, 0, len(triples))
	for _, t := range triples {
		lines = append(lines, t.Subject+" "+t.Predicate+" "+t.Object+".")
	}
	return strings.Join(lines, "\n"), nil
}

// Clear 清空图谱记忆.
// [EN] Clear the graph memory.
func (m *GraphMemory) Clear() {
	m.graph.Clear()
}

// sortedTriples 排序副本（召回结果确定性）.
// [EN] Sorted copy (deterministic recall).
func sortedTriples(ts []Triple) []Triple {
	out := append([]Triple(nil), ts...)
	sort.Slice(out, func(i, j int) bool { return tripleLess(out[i], out[j]) })
	return out
}

// tripleLess 三元组全序比较.
// [EN] Total order of triples.
func tripleLess(a, b Triple) bool {
	if a.Subject != b.Subject {
		return a.Subject < b.Subject
	}
	if a.Predicate != b.Predicate {
		return a.Predicate < b.Predicate
	}
	return a.Object < b.Object
}
