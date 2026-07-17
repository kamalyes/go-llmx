/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-17 21:18:36
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-17 21:18:36
 * @FilePath: \go-llmx\docstore\docstore.go
 * @Description: 文档键值存储 —— DocStore 契约与进程内实现.
 * 与 VectorStore（相似度检索）正交：DocStore 按 ID 精确存取完整文档，
 * 是 ParentDocumentRetriever（小块检索 → 父文档返回）的存储地基
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package docstore

import (
	"context"
	"errors"
	"fmt"
	"sync"

	llmx "github.com/kamalyes/go-llmx"
)

// ErrNotFound 键不存在（Get 未命中 / 删除后读取）.
// [EN] Key not found (Get miss / read after delete).
var ErrNotFound = errors.New("docstore: key not found")

// DocStore 文档键值存储（按 ID 存取完整文档）.
// [EN] Key-value document store (full documents by ID).
type DocStore interface {
	// Set 写入或覆盖文档（upsert 语义）.
	// [EN] Set or overwrite a document (upsert).
	Set(ctx context.Context, key string, doc llmx.Document) error

	// Get 读取文档（键不存在返回 ErrNotFound）.
	// [EN] Get a document (ErrNotFound on miss).
	Get(ctx context.Context, key string) (llmx.Document, error)

	// Delete 删除文档（键不存在静默成功，幂等）.
	// [EN] Delete a document (silent on miss, idempotent).
	Delete(ctx context.Context, key string) error

	// Keys 返回全部键（插入序）.
	// [EN] Return all keys (insertion order).
	Keys(ctx context.Context) []string
}

// InMemory 进程内实现（并发安全；重启即失，持久化场景接数据库后端）.
// [EN] In-process implementation (concurrency-safe; use a DB backend to persist).
type InMemory struct {
	mu sync.RWMutex

	// docs 键到文档的映射.
	// [EN] Key-to-document map.
	docs map[string]llmx.Document

	// order 揮入序键列表（Keys 稳定遍历）.
	// [EN] Insertion-ordered keys (stable iteration).
	order []string
}

// 编译期断言：InMemory 实现 DocStore 契约.
// [EN] Compile-time assertion.
var _ DocStore = (*InMemory)(nil)

// NewInMemory 构造进程内文档存储.
// [EN] Build an in-memory doc store.
func NewInMemory() *InMemory {
	return &InMemory{docs: make(map[string]llmx.Document)}
}

// Set 实现 DocStore（新键追加插入序，已存在键保序覆盖）.
// [EN] Implement DocStore (append new keys; overwrite in place).
func (s *InMemory) Set(_ context.Context, key string, doc llmx.Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.docs[key]; !exists {
		s.order = append(s.order, key)
	}
	s.docs[key] = doc
	return nil
}

// Get 实现 DocStore（键不存在包装 ErrNotFound 并注明键名）.
// [EN] Implement DocStore (miss wraps ErrNotFound with the key).
func (s *InMemory) Get(_ context.Context, key string) (llmx.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, ok := s.docs[key]
	if !ok {
		return llmx.Document{}, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return doc, nil
}

// Delete 实现 DocStore（保序移除；键不存在幂等成功）.
// [EN] Implement DocStore (order-preserving removal; idempotent).
func (s *InMemory) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.docs[key]; !ok {
		return nil
	}
	delete(s.docs, key)
	for i, k := range s.order {
		if k == key {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return nil
}

// Keys 实现 DocStore（返回插入序副本，外部修改不影响内部状态）.
// [EN] Implement DocStore (insertion-order copy; mutations don't leak in).
func (s *InMemory) Keys(_ context.Context) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}
