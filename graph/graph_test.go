/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-15 20:51:32
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-15 21:08:59
 * @FilePath: \go-llmx\graph\graph_test.go
 * @Description: 图谱核心单测 —— 增删查索引/邻居/去重/合并/
 * BFS/最短路/序列化/String 全路径覆盖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package graph

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleGraph 构造样例图谱（kamalyes 生态关系）.
// [EN] Build a sample graph.
func sampleGraph() *Graph {
	g := NewGraph()
	g.Add(
		Triple{Subject: "go-llmx", Predicate: "依赖", Object: "transport"},
		Triple{Subject: "go-llmx", Predicate: "依赖", Object: "go-logger"},
		Triple{Subject: "transport", Predicate: "依赖", Object: "go-logger"},
		Triple{Subject: "agent", Predicate: "组合", Object: "go-llmx"},
	)
	return g
}

// TestAddDedup 验证批量添加与去重.
// [EN] Verify batch add and dedup.
func TestAddDedup(t *testing.T) {
	g := NewGraph()
	assert.Equal(t, 1, g.Add(
		Triple{Subject: "a", Predicate: "p", Object: "b"},
		Triple{Subject: "a", Predicate: "p", Object: "b"},
	))
	assert.Equal(t, 0, g.Add(Triple{Subject: "a", Predicate: "p", Object: "b"}))
	assert.Equal(t, 1, g.Size())
}

// TestRemove 验证删除与索引一致性.
// [EN] Verify removal and index consistency.
func TestRemove(t *testing.T) {
	g := sampleGraph()
	t1 := Triple{Subject: "go-llmx", Predicate: "依赖", Object: "transport"}

	assert.True(t, g.Remove(t1))
	assert.False(t, g.Remove(t1))
	require.Len(t, g.TriplesBySubject("go-llmx"), 1)
	assert.Equal(t, "go-logger", g.TriplesBySubject("go-llmx")[0].Object)
	assert.Empty(t, g.TriplesByObject("transport"))
}

// TestEdgesQuery 验证出入边查询.
// [EN] Verify in/out edge queries.
func TestEdgesQuery(t *testing.T) {
	g := sampleGraph()

	subject := g.TriplesBySubject("go-llmx")
	require.Len(t, subject, 2)

	objs := g.TriplesByObject("go-logger")
	require.Len(t, objs, 2)
	for _, tr := range objs {
		assert.Contains(t, []string{"go-llmx", "transport"}, tr.Subject)
	}
}

// TestNeighborsNodes 验证邻居与全实体.
// [EN] Verify neighbors and all nodes.
func TestNeighborsNodes(t *testing.T) {
	g := sampleGraph()

	assert.Equal(t, []string{"agent", "go-logger", "transport"}, g.Neighbors("go-llmx"))
	assert.Equal(t, []string{"go-llmx", "transport"}, g.Neighbors("go-logger"))

	nodes := g.Nodes()
	assert.Equal(t, []string{"agent", "go-llmx", "go-logger", "transport"}, nodes)
}

// TestTriplesSorted 验证全量三元组排序确定性.
// [EN] Verify deterministic ordering of all triples.
func TestTriplesSorted(t *testing.T) {
	g := NewGraph()
	g.Add(
		Triple{Subject: "b", Predicate: "r", Object: "a"},
		Triple{Subject: "a", Predicate: "r", Object: "c"},
		Triple{Subject: "a", Predicate: "p", Object: "z"},
	)
	triples := g.Triples()
	require.Len(t, triples, 3)
	assert.Equal(t, "a", triples[0].Subject)
	assert.Equal(t, "p", triples[0].Predicate)
}

// TestMerge 验证图谱合并去重.
// [EN] Verify graph merge with dedup.
func TestMerge(t *testing.T) {
	g1 := sampleGraph()
	g2 := NewGraph()
	g2.Add(
		Triple{Subject: "go-llmx", Predicate: "依赖", Object: "transport"},
		Triple{Subject: "chain", Predicate: "组合", Object: "go-llmx"},
	)

	assert.Equal(t, 1, g1.Merge(g2))
	assert.Equal(t, 5, g1.Size())
	assert.Equal(t, 0, g1.Merge(nil))
}

// TestBFS 验证广度优先可达集（深度/排除）.
// [EN] Verify BFS reachability (depth/exclusion).
func TestBFS(t *testing.T) {
	g := sampleGraph()

	// go-llmx 1 跳（无向语义）：agent、transport、go-logger
	assert.Equal(t, []string{"agent", "go-llmx", "go-logger", "transport"}, g.BFS("go-llmx", 1))
	// 排除两个出边邻居后仅剩 agent（入边）
	assert.Equal(t, []string{"agent", "go-llmx"}, g.BFS("go-llmx", 2, "transport", "go-logger"))
	// 未知实体返回 nil
	assert.Nil(t, g.BFS("unknown", 3))
}

// TestShortestPath 验证最短路（无向语义）.
// [EN] Verify shortest paths (undirected semantics).
func TestShortestPath(t *testing.T) {
	g := sampleGraph()

	assert.Equal(t, []string{"go-llmx", "transport"}, g.ShortestPath("go-llmx", "transport"))
	assert.Equal(t, []string{"agent", "go-llmx", "go-logger"}, g.ShortestPath("agent", "go-logger"))
	assert.Equal(t, []string{"agent"}, g.ShortestPath("agent", "agent"))
	assert.Nil(t, g.ShortestPath("agent", "unknown"))
}

// TestString 验证 Turtle 风格输出.
// [EN] Verify Turtle-style output.
func TestString(t *testing.T) {
	g := NewGraph()
	g.Add(Triple{Subject: "a", Predicate: "likes", Object: "b"})
	assert.Equal(t, "a likes b.", g.String())
}

// TestJSONRoundTrip 验证序列化往返.
// [EN] Verify JSON round-trip.
func TestJSONRoundTrip(t *testing.T) {
	g := sampleGraph()
	data, err := json.Marshal(g)
	require.NoError(t, err)

	restored := NewGraph()
	require.NoError(t, json.Unmarshal(data, restored))
	assert.Equal(t, g.Triples(), restored.Triples())
	assert.Equal(t, g.String(), restored.String())
}

// TestUnmarshalInvalid 验证非法载荷报错.
// [EN] Verify error on invalid payloads.
func TestUnmarshalInvalid(t *testing.T) {
	g := NewGraph()
	assert.Error(t, json.Unmarshal([]byte(`{"not":"array"}`), g))
}

// TestConcurrency 验证并发读写安全.
// [EN] Verify concurrent read/write safety.
func TestConcurrency(t *testing.T) {
	g := NewGraph()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			g.Add(Triple{Subject: "s", Predicate: "p", Object: string(rune('a' + n%26))})
			_ = g.TriplesBySubject("s")
			_ = g.Neighbors("s")
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 26, g.Size())
}
