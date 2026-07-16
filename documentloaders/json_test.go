/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 20:33:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 20:33:58
 * @FilePath: \go-llmx\documentloaders\json_test.go
 * @Description: JSON 加载器测试 —— 顶层数组/点路径导航/内容字段提取/
 * 非字符串序列化/错误路径，纯字符串输入零依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documentloaders

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSON_TopLevelArray 验证顶层数组每元素一文档.
// [EN] Verify top-level arrays.
func TestJSON_TopLevelArray(t *testing.T) {
	data := `["first", "second", "third"]`
	docs, err := NewJSON(strings.NewReader(data)).Load(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 3)
	assert.Equal(t, "first", docs[0].PageContent)
	assert.Equal(t, 0, docs[0].Metadata["seq"])
	assert.Equal(t, 2, docs[2].Metadata["seq"])
}

// TestJSON_DotPathAndContentKey 验证点路径定位与内容字段提取.
// [EN] Verify dot paths and content keys.
func TestJSON_DotPathAndContentKey(t *testing.T) {
	data := `{"items":[{"content":"alpha","tag":"x"},{"content":"beta","tag":"y"}]}`
	docs, err := NewJSON(strings.NewReader(data)).
		WithPath("items").
		WithContentKey("content").
		Load(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 2)
	assert.Equal(t, "alpha", docs[0].PageContent)
	assert.Equal(t, "x", docs[0].Metadata["tag"]) // 其余字段入元数据
	assert.Equal(t, "beta", docs[1].PageContent)
}

// TestJSON_NonStringContent 验证非字符串内容序列化.
// [EN] Verify non-string content serialization.
func TestJSON_NonStringContent(t *testing.T) {
	data := `[{"content":{"deep":true}}]`
	docs, err := NewJSON(strings.NewReader(data)).WithContentKey("content").Load(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.JSONEq(t, `{"deep":true}`, docs[0].PageContent)
}

// TestJSON_PathNotArray 验证路径非数组报错.
// [EN] Verify non-array path errors.
func TestJSON_PathNotArray(t *testing.T) {
	_, err := NewJSON(strings.NewReader(`{"items":{"x":1}}`)).WithPath("items").Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an array")
}

// TestJSON_MissingPathSegment 验证路径缺段报错.
// [EN] Verify missing path segments error.
func TestJSON_MissingPathSegment(t *testing.T) {
	_, err := NewJSON(strings.NewReader(`{}`)).WithPath("items.data").Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `segment "items" missing`)
}

// TestJSON_InvalidSyntax 验证非法 JSON 报错.
// [EN] Verify invalid JSON errors.
func TestJSON_InvalidSyntax(t *testing.T) {
	_, err := NewJSON(strings.NewReader(`{broken`)).Load(context.Background())
	require.Error(t, err)
}
