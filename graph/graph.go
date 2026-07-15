/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-15 20:51:32
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-15 20:51:32
 * @FilePath: \go-llmx\graph\graph.go
 * @Description: 知识图谱核心 —— 三元组邻接表 + 出入边双向索引，
 * 实体度查询 O(degree)；BFS/最短路等图算法内建，
 * RWMutex 并发安全；String 输出 Turtle 风格三元组行便于 prompt 注入
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package graph

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Triple 知识三元组（subject predicate object）.
// [EN] Knowledge triple (subject predicate object).
type Triple struct {
	// Subject 主体实体.
	// [EN] Subject entity.
	Subject string `json:"subject"`

	// Predicate 谓词（关系名）.
	// [EN] Predicate (relation name).
	Predicate string `json:"predicate"`

	// Object 客体实体.
	// [EN] Object entity.
	Object string `json:"object"`
}

// key 三元组去重键.
// [EN] Deduplication key of a triple.
func (t Triple) key() string {
	return t.Subject + "\x00" + t.Predicate + "\x00" + t.Object
}

// Graph 知识图谱（三元组集合 + 出入边索引）.
// [EN] Knowledge graph (triple set + in/out edge indexes).
type Graph struct {
	// mu 并发保护（读多写少）.
	// [EN] Concurrency guard (read-mostly).
	mu sync.RWMutex

	// triples 三元组去重集合.
	// [EN] Deduplicated triple set.
	triples map[string]Triple

	// outEdges 主体出边索引（subject → 该实体为 subject 的三元组）.
	// [EN] Out-edge index (subject → triples with this subject).
	outEdges map[string][]Triple

	// inEdges 客体入边索引（object → 该实体为 object 的三元组）.
	// [EN] In-edge index (object → triples with this object).
	inEdges map[string][]Triple
}

// NewGraph 构造空图谱.
// [EN] Build an empty graph.
func NewGraph() *Graph {
	return &Graph{
		triples: map[string]Triple{},
		outEdges: map[string][]Triple{},
		inEdges:  map[string][]Triple{},
	}
}

// Add 批量添加三元组（去重，返回实际新增数量）.
// [EN] Add triples in batch (dedup; returns the added count).
func (g *Graph) Add(triples ...Triple) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	added := 0
	for _, t := range triples {
		k := t.key()
		if _, ok := g.triples[k]; ok {
			continue
		}
		g.triples[k] = t
		g.outEdges[t.Subject] = append(g.outEdges[t.Subject], t)
		g.inEdges[t.Object] = append(g.inEdges[t.Object], t)
		added++
	}
	return added
}

// Remove 删除三元组（返回是否存在）.
// [EN] Remove a triple (reports existence).
func (g *Graph) Remove(t Triple) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	k := t.key()
	if _, ok := g.triples[k]; !ok {
		return false
	}
	delete(g.triples, k)
	g.outEdges[t.Subject] = removeTriple(g.outEdges[t.Subject], t)
	g.inEdges[t.Object] = removeTriple(g.inEdges[t.Object], t)
	return true
}

// removeTriple 从切片剔除等值三元组（索引内存有效，线性可接受）.
// [EN] Drop the equal triple from a slice (in-memory, linear is fine).
func removeTriple(list []Triple, t Triple) []Triple {
	for i, x := range list {
		if x == t {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// TriplesBySubject 查询主体的全部出边三元组.
// [EN] All out-edge triples of a subject.
func (g *Graph) TriplesBySubject(subject string) []Triple {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return append([]Triple(nil), g.outEdges[subject]...)
}

// TriplesByObject 查询客体的全部入边三元组.
// [EN] All in-edge triples of an object.
func (g *Graph) TriplesByObject(object string) []Triple {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return append([]Triple(nil), g.inEdges[object]...)
}

// Neighbors 实体的直接邻居（出边客体 + 入边主体，去重排序）.
// [EN] Direct neighbors (out-edge objects + in-edge subjects, dedup + sorted).
func (g *Graph) Neighbors(entity string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	set := map[string]struct{}{}
	for _, t := range g.outEdges[entity] {
		set[t.Object] = struct{}{}
	}
	for _, t := range g.inEdges[entity] {
		set[t.Subject] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Nodes 全部实体（去重排序）.
// [EN] All entities (dedup + sorted).
func (g *Graph) Nodes() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	set := map[string]struct{}{}
	for _, t := range g.triples {
		set[t.Subject] = struct{}{}
		set[t.Object] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Triples 全部三元组（副本，键排序确定性）.
// [EN] All triples (a copy, key-sorted deterministically).
func (g *Graph) Triples() []Triple {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]Triple, 0, len(g.triples))
	for _, t := range g.triples {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		if a.Predicate != b.Predicate {
			return a.Predicate < b.Predicate
		}
		return a.Object < b.Object
	})
	return out
}

// Size 三元组数量.
// [EN] Triple count.
func (g *Graph) Size() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.triples)
}

// Merge 合并另一图谱（去重，返回实际新增数量）.
// [EN] Merge another graph (dedup; returns the added count).
func (g *Graph) Merge(other *Graph) int {
	if other == nil {
		return 0
	}
	return g.Add(other.Triples()...)
}

// BFS 从起点按谓词不限广度优先收集可达实体（含起点，跳过 exclude）.
// [EN] Breadth-first reachable entities from a start (inclusive; skip exclude).
func (g *Graph) BFS(start string, depth int, exclude ...string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if _, ok := g.outEdges[start]; !ok {
		if _, ok := g.inEdges[start]; !ok {
			return nil
		}
	}

	skip := map[string]struct{}{}
	for _, e := range exclude {
		skip[e] = struct{}{}
	}
	seen := map[string]struct{}{start: {}}
	frontier := []string{start}
	reached := []string{start}
	for d := 0; d < depth && len(frontier) > 0; d++ {
		next := make([]string, 0, len(frontier))
		for _, node := range frontier {
			for _, n := range g.unlockedNeighbors(node) {
				if _, ok := seen[n]; ok {
					continue
				}
				if _, ok := skip[n]; ok {
					continue
				}
				seen[n] = struct{}{}
				next = append(next, n)
				reached = append(reached, n)
			}
		}
		frontier = next
	}
	sort.Strings(reached)
	return reached
}

// unlockedNeighbors 邻居查询（调用方已持读锁）.
// [EN] Neighbor query (caller already holds the read lock).
func (g *Graph) unlockedNeighbors(entity string) []string {
	set := map[string]struct{}{}
	for _, t := range g.outEdges[entity] {
		set[t.Object] = struct{}{}
	}
	for _, t := range g.inEdges[entity] {
		set[t.Subject] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	return out
}

// ShortestPath 起终点最短路径（BFS，无向语义；不可达返回 nil）.
// [EN] Shortest path between two nodes (BFS, undirected semantics; nil if unreachable).
func (g *Graph) ShortestPath(from, to string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if from == to {
		if g.unlockedHasNode(from) {
			return []string{from}
		}
		return nil
	}
	prev := map[string]string{from: ""}
	queue := []string{from}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, n := range g.unlockedNeighbors(node) {
			if _, ok := prev[n]; ok {
				continue
			}
			prev[n] = node
			if n == to {
				path := []string{to}
				for p := node; p != ""; p = prev[p] {
					path = append([]string{p}, path...)
				}
				return path
			}
			queue = append(queue, n)
		}
	}
	return nil
}

// unlockedHasNode 实体存在性（调用方已持读锁）.
// [EN] Entity existence (caller already holds the read lock).
func (g *Graph) unlockedHasNode(entity string) bool {
	_, out := g.outEdges[entity]
	_, in := g.inEdges[entity]
	return out || in
}

// Clear 清空图谱.
// [EN] Clear the graph.
func (g *Graph) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.triples = map[string]Triple{}
	g.outEdges = map[string][]Triple{}
	g.inEdges = map[string][]Triple{}
}

// String Turtle 风格三元组行（键排序确定性，prompt 注入友好）.
// [EN] Turtle-style triple lines (key-sorted, prompt friendly).
func (g *Graph) String() string {
	lines := make([]string, 0, g.Size())
	for _, t := range g.Triples() {
		lines = append(lines, fmt.Sprintf("%s %s %s.", t.Subject, t.Predicate, t.Object))
	}
	return strings.Join(lines, "\n")
}

// MarshalJSON 序列化为三元组数组（排序确定性）.
// [EN] Serialize as a sorted triple array.
func (g *Graph) MarshalJSON() ([]byte, error) {
	return json.Marshal(g.Triples())
}

// UnmarshalJSON 从三元组数组恢复.
// [EN] Restore from a triple array.
func (g *Graph) UnmarshalJSON(data []byte) error {
	var triples []Triple
	if err := json.Unmarshal(data, &triples); err != nil {
		return err
	}
	g.Clear()
	g.Add(triples...)
	return nil
}
