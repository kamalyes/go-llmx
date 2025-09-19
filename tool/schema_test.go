/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-19 22:02:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-19 22:02:00
 * @FilePath: \go-llmx\tool\schema_test.go
 * @Description: 极简 JSON Schema 生成测试 —— 类型映射/嵌套/omitempty/指针/切片/map
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package tool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaArgs 测试用参数结构体.
// [EN] Argument struct for tests.
type schemaArgs struct {
	City   string   `json:"city"`
	Days   int      `json:"days"`
	Ratio  *float64 `json:"ratio,omitempty"`
	Tags   []string `json:"tags"`
	Extra  map[string]int
	hidden int // 未导出字段应被跳过
}

// nestedArgs 嵌套结构体测试.
// [EN] Nested struct case.
type nestedArgs struct {
	Location struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location"`
	Zoom int `json:"zoom,omitempty"`
}

func TestSchemaOf_Struct(t *testing.T) {
	s := SchemaOf(schemaArgs{})

	assert.Equal(t, "object", s["type"])
	props := s["properties"].(map[string]any)
	assert.Equal(t, map[string]any{"type": "string"}, props["city"])
	assert.Equal(t, map[string]any{"type": "number"}, props["days"])
	assert.Equal(t, map[string]any{"type": "number"}, props["ratio"]) // 指针解引用
	assert.Equal(t, map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, props["tags"])
	assert.Equal(t, map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "number"}}, props["Extra"])

	// city/days/tags/Extra 必填，ratio omitempty 不在 required
	required := s["required"].([]string)
	assert.Contains(t, required, "city")
	assert.Contains(t, required, "tags")
	assert.NotContains(t, required, "ratio")
	// 未导出字段不出现
	assert.NotContains(t, props, "hidden")
}

func TestSchemaOf_NestedStruct(t *testing.T) {
	s := SchemaOf(nestedArgs{})
	props := s["properties"].(map[string]any)
	loc := props["location"].(map[string]any)
	assert.Equal(t, "object", loc["type"])
	locProps := loc["properties"].(map[string]any)
	assert.Equal(t, map[string]any{"type": "number"}, locProps["lat"])
}

func TestSchemaOf_BasicKinds(t *testing.T) {
	assert.Equal(t, map[string]any{"type": "string"}, SchemaOf("x"))
	assert.Equal(t, map[string]any{"type": "number"}, SchemaOf(42))
	assert.Equal(t, map[string]any{"type": "number"}, SchemaOf(uint8(1)))
	assert.Equal(t, map[string]any{"type": "number"}, SchemaOf(3.14))
	assert.Equal(t, map[string]any{"type": "boolean"}, SchemaOf(true))
	assert.Equal(t, map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, SchemaOf([]string{}))
	assert.Equal(t, map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}}, SchemaOf(map[string]string{}))
}

func TestSchemaOf_PointerWrap(t *testing.T) {
	// 指针包裹的 struct 等价于 struct 本身
	s := SchemaOf(&schemaArgs{})
	assert.Equal(t, "object", s["type"])
}

func TestSchemaOf_UnknownKind(t *testing.T) {
	// nil / 无法识别的类型 → 空 schema 宽松容错
	assert.Empty(t, SchemaOf(nil))
	assert.Empty(t, SchemaOf(func() {}))
	// [5]int 数组走 array
	assert.Equal(t, map[string]any{"type": "array", "items": map[string]any{"type": "number"}}, SchemaOf([5]int{}))
}

func TestSchemaOf_JsonTagNaming(t *testing.T) {
	type tagged struct {
		Name string `json:"display_name"`
		Age  int    `json:"-"`
	}
	s := SchemaOf(tagged{})
	props := s["properties"].(map[string]any)
	require.Contains(t, props, "display_name")
	assert.NotContains(t, props, "Name")
	assert.NotContains(t, props, "Age")
}
