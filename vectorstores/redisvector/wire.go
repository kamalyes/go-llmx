/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-12 21:12:26
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-12 21:23:07
 * @FilePath: \go-llmx\vectorstores\redisvector\wire.go
 * @Description: RediSearch 协议编解码 —— 向量 FLOAT32 小端 blob、
 * FT.CREATE / KNN FT.SEARCH 参数构造、嵌套数组响应解析.
 * 字段名与键前缀对齐 langchaingo/redisvector，存量数据平滑读写
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcredisvector

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"

	llmx "github.com/kamalyes/go-llmx"
)

// encodeVector 编码向量为 FLOAT32 小端 blob（RediSearch VECTOR 协议形态）.
// [EN] Encode a vector as a little-endian FLOAT32 blob.
func encodeVector(v []float64) []byte {
	b := make([]byte, len(v)*4)
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(float32(x)))
	}
	return b
}

// buildCreateArgs 构造 FT.CREATE 参数（HNSW 余弦 + content TEXT + 过滤字段 TAG）.
// [EN] Build FT.CREATE args (HNSW cosine + content TEXT + TAG filter fields).
func buildCreateArgs(index, prefix string, dim int, filterFields []string) []any {
	args := []any{
		"FT.CREATE", index,
		"ON", "HASH",
		"PREFIX", 1, prefix + ":",
		"SCHEMA",
		fieldContent, "TEXT",
		fieldVector, "VECTOR", "HNSW", 6,
		"TYPE", hnswType,
		"DIM", dim,
		"DISTANCE_METRIC", "COSINE",
		"M", hnswM,
		"EF_CONSTRUCTION", hnswEFConstant,
		"EF_RUNTIME", hnswEFRuntime,
	}
	for _, f := range filterFields {
		args = append(args, f, "TAG")
	}
	return args
}

// buildKNNQuery 构造 KNN 查询串（TAG 过滤前缀 + KNN 子句，langchaingo 同构）.
// [EN] Build the KNN query string (TAG filter prefix + KNN clause).
//
// 无过滤：*=>[KNN k @content_vector $vec AS distance]
// 有过滤：(@a:{x} @b:{y})=>[KNN k @content_vector $vec AS distance]
func buildKNNQuery(topK int, filters []llmx.Filter) string {
	if topK <= 0 {
		topK = 10
	}
	var pre string
	for _, f := range filters {
		for k, v := range f {
			if pre == "" {
				pre = "(@"
			} else {
				pre += " @"
			}
			pre += k + ":{" + strValue(v) + "}"
		}
	}
	if pre != "" {
		pre += ")"
	} else {
		pre = "*"
	}
	return pre + "=>[KNN " + strconv.Itoa(topK) + " @" + fieldVector + " $vec AS " + fieldScore + "]"
}

// buildSearchArgs 构造 FT.SEARCH 参数（RETURN 裁剪响应避免回传向量 blob）.
// [EN] Build FT.SEARCH args (RETURN trims the response, skipping the vector blob).
//
// RETURN 只取 content 与已知元数据字段——1536 维 blob 约 6KB/条，
// 裁剪后响应体积下降一个数量级
func buildSearchArgs(index, query string, vec []float64, topK int, returnFields []string) []any {
	if topK <= 0 {
		topK = 10
	}
	args := make([]any, 0, 16+len(returnFields)*2)
	args = append(args,
		"FT.SEARCH", index, query,
		"RETURN", 1+len(returnFields), fieldContent,
	)
	for _, f := range returnFields {
		args = append(args, f)
	}
	args = append(args,
		"SORTBY", fieldScore, "ASC",
		"LIMIT", 0, topK,
		"PARAMS", 2, "vec", encodeVector(vec),
		"DIALECT", 2,
	)
	return args
}

// parseSearchResponse 解析 FT.SEARCH 嵌套数组响应.
// [EN] Parse the nested-array FT.SEARCH response.
//
// 形态：[total, doc1, doc2...]，doc = [key, [field, value, ...]]；
// content→PageContent，其余字段→Metadata，key→metadata["id"]（langchaingo 语义）
func parseSearchResponse(v any) []llmx.Document {
	arr, ok := v.([]any)
	if !ok || len(arr) < 2 {
		return nil
	}
	docs := make([]llmx.Document, 0, len(arr)-1)
	for _, d := range arr[1:] {
		da, ok := d.([]any)
		if !ok || len(da) < 2 {
			continue
		}
		var doc llmx.Document
		if key, ok := da[0].(string); ok {
			doc.Metadata = map[string]any{"id": key}
		}
		fields, _ := da[1].([]any)
		for i := 0; i+1 < len(fields); i += 2 {
			name, _ := fields[i].(string)
			if name == fieldContent {
				doc.PageContent, _ = fields[i+1].(string)
				continue
			}
			if doc.Metadata == nil {
				doc.Metadata = map[string]any{}
			}
			if val, ok := fields[i+1].(string); ok {
				doc.Metadata[name] = val
			}
		}
		docs = append(docs, doc)
	}
	return docs
}

// strValue 元数据值字符串化（langchaingo 平铺语义 fmt.Sprint 同构）.
// [EN] Stringify a metadata value (same as langchaingo flattening).
func strValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}
