/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-19 21:29:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-19 21:29:00
 * @FilePath: \go-llmx\tool\schema.go
 * @Description: 极简 JSON Schema 生成 —— Go 结构体 → Tool.Parameters.
 * 自研零依赖（砍掉 langchaingo 的 kin-openapi 传递依赖），
 * 覆盖 string/number/boolean/array/map/嵌套结构体/指针
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package tool

import (
	"reflect"
	"strings"
)

// SchemaOf 从 Go 值生成极简 JSON Schema（struct 字段 → object properties）.
// [EN] Generate a minimal JSON Schema from a Go value.
//
// 映射规则：string→string；整数/浮点→number；bool→boolean；
// 切片→array+items；map→object+additionalProperties；嵌套结构体递归；
// 指针解引用。带 omitempty 标签的字段不进 required，其余字段默认必填
func SchemaOf(v any) map[string]any {
	return schemaOf(reflect.TypeOf(v))
}

// schemaOf 类型递归（nil/无法识别的类型返回空 schema，宽松容错）.
// [EN] Recursive typing (lenient: empty schema for nil/unknown kinds).
func schemaOf(t reflect.Type) map[string]any {
	if t == nil {
		return map[string]any{}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		return structSchema(t)
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schemaOf(t.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schemaOf(t.Elem())}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	default:
		return map[string]any{}
	}
}

// structSchema 结构体字段展开（json 标签命名 + omitempty 可选）.
// [EN] Expand struct fields (json tag naming + omitempty optional).
func structSchema(t reflect.Type) map[string]any {
	props := map[string]any{}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, optional := f.Name, false
		if tag, ok := f.Tag.Lookup("json"); ok {
			parts := strings.Split(tag, ",")
			if parts[0] == "-" {
				continue // json:"-" 字段完全剔除
			}
			if parts[0] != "" {
				name = parts[0]
			}
			for _, p := range parts[1:] {
				if p == "omitempty" {
					optional = true
				}
			}
		}
		props[name] = schemaOf(f.Type)
		if !optional {
			required = append(required, name)
		}
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
