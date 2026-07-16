/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 22:58:11
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 22:58:11
 * @FilePath: \go-llmx\tool\sqldatabase_test.go
 * @Description: SQL 工具测试 —— modernc 纯 Go SQLite 内存库，
 * 覆盖查询/截断/只读防护/多语句拒绝，无 CGO 零网络
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package tool

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDB 内存 SQLite（每用例独立）.
// [EN] Per-case in-memory SQLite.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);
		INSERT INTO users (name) VALUES ('alice'), ('bob'), ('carol')`)
	require.NoError(t, err)
	return db
}

// TestSQLDatabase_Query 验证 SELECT 结果 JSON 行.
// [EN] Verify SELECT result rows.
func TestSQLDatabase_Query(t *testing.T) {
	tool := NewSQLDatabase(newTestDB(t), 10)
	out, err := tool.Func(context.Background(), `{"query":"SELECT id, name FROM users WHERE id = 1"}`)
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "alice", rows[0]["name"])
}

// TestSQLDatabase_Truncates 验证行数截断标记.
// [EN] Verify row truncation.
func TestSQLDatabase_Truncates(t *testing.T) {
	tool := NewSQLDatabase(newTestDB(t), 2)
	out, err := tool.Func(context.Background(), `{"query":"SELECT id FROM users"}`)
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &rows))
	require.Len(t, rows, 3) // 2 数据行 + 1 截断标记行
	assert.Equal(t, true, rows[2]["_truncated"])
}

// TestSQLDatabase_RejectsWrites 验证写语句只读防护.
// [EN] Verify the read-only guard.
func TestSQLDatabase_RejectsWrites(t *testing.T) {
	tool := NewSQLDatabase(newTestDB(t), 10)
	for _, q := range []string{
		"DELETE FROM users",
		"DROP TABLE users",
		"INSERT INTO users (name) VALUES ('x')",
		"UPDATE users SET name = 'x'",
	} {
		_, err := tool.Func(context.Background(), `{"query":"`+q+`"}`)
		assert.ErrorIs(t, err, llmx.ErrUnsupportedOperation, q)
	}
}

// TestSQLDatabase_RejectsMultiStatement 验证多语句拒绝.
// [EN] Verify multi-statement rejection.
func TestSQLDatabase_RejectsMultiStatement(t *testing.T) {
	tool := NewSQLDatabase(newTestDB(t), 10)
	_, err := tool.Func(context.Background(), `{"query":"SELECT 1; DELETE FROM users"}`)
	assert.ErrorIs(t, err, llmx.ErrUnsupportedOperation)
}

// TestSQLDatabase_EmptyQuery 验证空查询报错.
// [EN] Verify empty queries error.
func TestSQLDatabase_EmptyQuery(t *testing.T) {
	tool := NewSQLDatabase(newTestDB(t), 10)
	_, err := tool.Func(context.Background(), `{"query":"  "}`)
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}

// TestSQLDatabase_SyntaxError 验证 SQL 语法错误透传.
// [EN] Verify SQL syntax errors propagate.
func TestSQLDatabase_SyntaxError(t *testing.T) {
	tool := NewSQLDatabase(newTestDB(t), 10)
	_, err := tool.Func(context.Background(), `{"query":"SELECT FROM WHERE"}`)
	require.Error(t, err)
}
