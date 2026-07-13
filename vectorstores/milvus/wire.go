/*
 * @Author: wmxuan 836551135@qq.com
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-13 23:07:26
 * @FilePath: \go-llmx\vectorstores\milvus\wire.go
 * @Description: Milvus RESTful v2 协议层 —— 集合 schema/检索请求构造、
 * 过滤表达式（元数据 JSON 路径引用）、响应状态与行集还原
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmilvus

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// wireResponse RESTful v2 统一响应头.
// [EN] Common RESTful v2 response head.
type wireResponse struct {
	// Code 状态码（0 成功）.
	// [EN] Status code (0 for success).
	Code int `json:"code"`

	// Message 错误描述.
	// [EN] Error message.
	Message string `json:"message"`
}

// asError 非零码转错误.
// [EN] Non-zero code to error.
func (w wireResponse) asError() error {
	if w.Code == codeSuccess {
		return nil
	}
	return fmt.Errorf("llmx: milvus code=%d: %s", w.Code, w.Message)
}

// wireField schema 字段定义.
// [EN] Schema field definition.
type wireField struct {
	FieldName         string            `json:"fieldName"`
	DataType          string            `json:"dataType"`
	IsPrimary         bool              `json:"isPrimary,omitempty"`
	AutoID            bool              `json:"autoID,omitempty"`
	ElementTypeParams map[string]string `json:"elementTypeParams,omitempty"`
}

// wireSchema 集合 schema.
// [EN] Collection schema.
type wireSchema struct {
	AutoID bool         `json:"autoId"`
	Fields []*wireField `json:"fields"`
}

// wireIndexParam 索引参数.
// [EN] Index parameter.
type wireIndexParam struct {
	FieldName  string `json:"fieldName"`
	IndexName  string `json:"indexName"`
	MetricType string `json:"metricType"`
}

// wireCreateRequest 建集合请求.
// [EN] Create-collection request.
type wireCreateRequest struct {
	CollectionName string            `json:"collectionName"`
	Schema         *wireSchema       `json:"schema"`
	IndexParams    []*wireIndexParam `json:"indexParams"`
}

// buildCreateRequest 构造建集合请求（pk 自增 + text/meta + 浮点向量 + AUTOINDEX）.
// [EN] Build the create request (auto pk + text/meta + float vector + AUTOINDEX).
func buildCreateRequest(s *Store) *wireCreateRequest {
	return &wireCreateRequest{
		CollectionName: s.collectionName,
		Schema: &wireSchema{
			AutoID: s.autoID,
			Fields: []*wireField{
				{FieldName: s.primaryField, DataType: dataTypeInt64, IsPrimary: true, AutoID: s.autoID},
				{FieldName: s.textField, DataType: dataTypeVarChar, ElementTypeParams: map[string]string{
					typeParamMaxLength: strconv.Itoa(s.maxTextLength)}},
				{FieldName: s.metaField, DataType: dataTypeJSON},
				{FieldName: s.vectorField, DataType: dataTypeFloatVector, ElementTypeParams: map[string]string{
					typeParamDim: strconv.Itoa(s.dim)}},
			},
		},
		IndexParams: []*wireIndexParam{{
			FieldName:  s.vectorField,
			IndexName:  defaultIndexName,
			MetricType: s.metricType,
		}},
	}
}

// wireHasRequest / wireHasData 存在性探测.
// [EN] Existence probe.
type wireHasRequest struct {
	CollectionName string `json:"collectionName"`
}

type wireHasData struct {
	Has bool `json:"has"`
}

// wireHasResponse 存在性响应.
// [EN] Existence response.
type wireHasResponse struct {
	wireResponse
	Data *wireHasData `json:"data"`
}

// wireDescribeRequest / 响应（存量集合维度与加载态）.
// [EN] Describe request/response (dimension + load state).
type wireDescribeRequest struct {
	CollectionName string `json:"collectionName"`
}

type wireDescribeData struct {
	Fields []*wireField `json:"fields"`
	Load   string       `json:"load"`
}

type wireDescribeResponse struct {
	wireResponse
	Data *wireDescribeData `json:"data"`
}

// vectorDimFromDescribe 从 describe 提取向量维度.
// [EN] Extract the vector dimension from describe.
func vectorDimFromDescribe(d *wireDescribeData, vectorField string) (int, error) {
	if d == nil {
		return 0, fmt.Errorf("%w: empty describe", llmx.ErrInvalidRequest)
	}
	for _, f := range d.Fields {
		if f.FieldName == vectorField {
			if s, ok := f.ElementTypeParams[typeParamDim]; ok {
				n, err := strconv.Atoi(s)
				if err == nil && n > 0 {
					return n, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("%w: dimension of %q not found", llmx.ErrInvalidRequest, vectorField)
}

// wireLoadRequest 加载集合.
// [EN] Load collection.
type wireLoadRequest struct {
	CollectionName string `json:"collectionName"`
}

// wireInsertRequest 行式写入.
// [EN] Row-based insert.
type wireInsertRequest struct {
	CollectionName string           `json:"collectionName"`
	Data           []map[string]any `json:"data"`
}

// wireInsertData 写入结果.
// [EN] Insert result.
type wireInsertData struct {
	InsertCount int `json:"insertCount"`
}

// wireInsertResponse 写入响应.
// [EN] Insert response.
type wireInsertResponse struct {
	wireResponse
	Data *wireInsertData `json:"data"`
}

// wireFlushRequest 刷盘.
// [EN] Flush.
type wireFlushRequest struct {
	CollectionNames []string `json:"collectionNames"`
}

// wireSearchRequest 检索请求.
// [EN] Search request.
type wireSearchRequest struct {
	CollectionName string            `json:"collectionName"`
	Data           [][]float64       `json:"data"`
	AnnsField      string            `json:"annsField"`
	Limit          int               `json:"limit"`
	OutputFields   []string          `json:"outputFields"`
	Filter         string            `json:"filter,omitempty"`
	SearchParams   map[string]string `json:"searchParams"`
}

// buildSearchRequest 构造检索请求（data 为单查询向量）.
// [EN] Build the search request (single query vector).
func buildSearchRequest(s *Store, query []float64, topK int, filters []llmx.Filter) *wireSearchRequest {
	if topK <= 0 {
		topK = defaultTopK
	}
	return &wireSearchRequest{
		CollectionName: s.collectionName,
		Data:           [][]float64{query},
		AnnsField:      s.vectorField,
		Limit:          topK,
		OutputFields:   []string{s.textField, s.metaField},
		Filter:         buildFilterExpr(s.metaField, filters),
		SearchParams:   map[string]string{"metricType": s.metricType},
	}
}

// wireSearchResponse 检索响应（行内含 text/meta 与 distance）.
// [EN] Search response (rows carry text/meta and distance).
type wireSearchResponse struct {
	wireResponse
	Data []map[string]any `json:"data"`
}

// rowsToDocuments 检索行还原为文档（distance 丢弃，meta 兼容对象/字符串）.
// [EN] Restore rows to documents (distance dropped; meta as object/string).
func rowsToDocuments(rows []map[string]any, textField, metaField string) []llmx.Document {
	if len(rows) == 0 {
		return nil
	}
	docs := make([]llmx.Document, 0, len(rows))
	for _, row := range rows {
		doc := llmx.Document{
			PageContent: asString(row[textField]),
		}
		switch meta := row[metaField].(type) {
		case map[string]any:
			doc.Metadata = meta
		case string:
			if meta != "" && meta != "null" {
				_ = unmarshalMeta(meta, &doc.Metadata)
			}
		}
		docs = append(docs, doc)
	}
	return docs
}

// buildFilterExpr 构造过滤表达式（meta["k"] == v，多条件 AND）.
// [EN] Build the filter expression (meta["k"] == v, AND-joined).
//
// 键排序保证表达式确定（map 迭代序随机）；字符串值转义双引号
func buildFilterExpr(metaField string, filters []llmx.Filter) string {
	if len(filters) == 0 {
		return ""
	}
	type kv struct{ k, v string }
	parts := make([]kv, 0, len(filters))
	for _, f := range filters {
		for k, v := range f {
			parts = append(parts, kv{k, quoteValue(v)})
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].k < parts[j].k })
	exprs := make([]string, 0, len(parts))
	for _, p := range parts {
		exprs = append(exprs, fmt.Sprintf("%s[%q] == %s", metaField, p.k, p.v))
	}
	return strings.Join(exprs, " && ")
}

// quoteValue 值字面量化（字符串加引号并转义，数字/布尔裸写）.
// [EN] Value literal (quoted-escaped strings; bare numbers/bools).
func quoteValue(v any) string {
	switch x := v.(type) {
	case string:
		return `"` + strings.ReplaceAll(x, `"`, `\"`) + `"`
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}

// asString 宽松字符串还原.
// [EN] Lenient string restoration.
func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

// unmarshalMeta JSON 元数据还原（失败静默容忍）.
// [EN] JSON metadata restore (silent tolerance).
func unmarshalMeta(s string, dst *map[string]any) error {
	return json.Unmarshal([]byte(s), dst)
}
