/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-29 22:27:39
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-29 22:39:36
 * @FilePath: \go-llmx\documentloaders\directory.go
 * @Description: 目录加载器 —— 递归匹配后缀加载文本文件（.md/.txt 常用），
 * 元数据携带相对路径；子目录深度优先稳定序
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documentloaders

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/textsplitter"
)

// Directory 目录加载器.
// [EN] Directory loader.
type Directory struct {
	// root 根目录.
	// [EN] Root directory.
	root string

	// exts 匹配后缀（小写含点，如 .md；空集默认 .md/.txt）.
	// [EN] Matched extensions (lowercase with dot; defaults .md/.txt).
	exts []string
}

// NewDirectory 构造目录加载器（exts 空集默认 .md/.txt）.
// [EN] Build a directory loader (defaults .md/.txt).
func NewDirectory(root string, exts ...string) *Directory {
	if len(exts) == 0 {
		exts = []string{".md", ".txt"}
	}
	normalized := make([]string, len(exts))
	for i, e := range exts {
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		normalized[i] = strings.ToLower(e)
	}
	return &Directory{root: root, exts: normalized}
}

// Load 实现 Loader（遍历全部匹配文件）.
// [EN] Implement Loader (all matching files).
func (l *Directory) Load(ctx context.Context) ([]llmx.Document, error) {
	if info, err := os.Stat(l.root); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: not a directory: %s", llmx.ErrInvalidRequest, l.root)
	}

	var paths []string
	err := filepath.WalkDir(l.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if l.matches(filepath.Ext(path)) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", l.root, err)
	}
	sort.Strings(paths)

	docs := make([]llmx.Document, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		rel, err := filepath.Rel(l.root, path)
		if err != nil {
			rel = path
		}
		docs = append(docs, llmx.Document{
			PageContent: string(data),
			Metadata: map[string]any{
				"source": rel,
			},
		})
	}
	return docs, nil
}

// LoadAndSplit 实现 Loader.
// [EN] Implement Loader.
func (l *Directory) LoadAndSplit(ctx context.Context, splitter textsplitter.Splitter) ([]llmx.Document, error) {
	return loadAndSplit(ctx, l, splitter)
}

// matches 后缀匹配（大小写不敏感）.
// [EN] Extension match (case-insensitive).
func (l *Directory) matches(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range l.exts {
		if ext == e {
			return true
		}
	}
	return false
}
