/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-16 22:52:36
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-16 22:52:36
 * @FilePath: \go-llmx\tool\sqldatabase.go
 * @Description: SQL 查询工具 —— 标准库 database/sql 之上的只读 SELECT，
 * 结果 JSON 行数组；零新驱动依赖（driver 由调用方注册）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package tool

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// sqlQueryArgs SQL 查询参数 Schema.
// [EN] SQL query argument schema.
type sqlQueryArgs struct {
	Query string `json:"query"`
}

// DefaultMaxSQLRows 结果行数上限（防巨结果撑爆上下文）.
// [EN] Row cap (guards the context window).
const DefaultMaxSQLRows = 100

// NewSQLDatabase SQL 查询工具：只接受单条 SELECT（只读防写），
// 结果截断 maxRows 行，列名小写化 JSON.
// [EN] SQL query tool: single read-only SELECT, row-capped.
func NewSQLDatabase(db *sql.DB, maxRows int, opts ...SQLOption) Tool {
	if maxRows <= 0 {
		maxRows = DefaultMaxSQLRows
	}
	s := &sqlState{db: db, maxRows: maxRows}
	for _, o := range opts {
		o(s)
	}
	return Tool{
		Name:        "sql_database",
		Description: "Run a read-only SQL SELECT against the database and return rows as JSON.",
		Parameters:  SchemaOf(sqlQueryArgs{}),
		Func:        s.query,
	}
}

// sqlState SQL 工具状态.
// [EN] SQL tool state.
type sqlState struct {
	db      *sql.DB
	maxRows int
}

// SQLOption SQL 工具配置.
// [EN] SQL tool option.
type SQLOption func(*sqlState)

// sqlWriteVerbs 拒绝的写操作前缀.
// [EN] Rejected write prefixes.
var sqlWriteVerbs = []string{
	"insert", "update", "delete", "drop", "create", "alter",
	"truncate", "replace", "merge", "grant", "revoke", "call", "exec",
}

// query 执行只读查询.
// [EN] Execute the read-only query.
func (s *sqlState) query(ctx context.Context, arguments string) (string, error) {
	args, err := unmarshalToolArgs[sqlQueryArgs](arguments)
	if err != nil {
		return "", err
	}
	q := strings.TrimSpace(args.Query)
	if q == "" {
		return "", fmt.Errorf("%w: empty query", llmx.ErrInvalidRequest)
	}

	// 只读防护：非 SELECT 一律拒绝；单语句限制防拼接写
	// [EN] Read-only guard: reject non-SELECT; single statement only.
	lower := strings.ToLower(q)
	for _, verb := range sqlWriteVerbs {
		if strings.HasPrefix(lower, verb) {
			return "", fmt.Errorf("%w: read-only tool rejects %q statements", llmx.ErrUnsupportedOperation, verb)
		}
	}
	if strings.Contains(lower, ";") && strings.TrimSpace(strings.SplitN(q, ";", 2)[1]) != "" {
		return "", fmt.Errorf("%w: multi-statement queries rejected", llmx.ErrUnsupportedOperation)
	}

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	out := make([]map[string]any, 0, s.maxRows)
	for rows.Next() {
		if len(out) >= s.maxRows {
			out = append(out, map[string]any{"_truncated": true, "_maxRows": s.maxRows})
			break
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = normalizeSQLValue(vals[i])
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// normalizeSQLValue []byte 列转字符串（驱动返回形态差异归一）.
// [EN] Normalize []byte columns to strings.
func normalizeSQLValue(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}
