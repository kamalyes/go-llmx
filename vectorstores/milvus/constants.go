/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-13 22:51:07
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-13 22:51:07
 * @FilePath: \go-llmx\vectorstores\milvus\constants.go
 * @Description: Milvus 常量 —— RESTful v2 端点前缀、集合/字段名与
 * langchaingo/milvus 默认对齐（pk/text/meta/vector），存量集合平滑读写
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmilvus

const (
	// DefaultBaseURL 默认 Milvus RESTful v2 地址.
	// [EN] Default Milvus RESTful v2 address.
	DefaultBaseURL = "http://localhost:19530"

	// apiVersion RESTful v2 前缀.
	// [EN] RESTful v2 prefix.
	apiVersion = "/v2/vectordb"
)

// REST 端点字面量.
// [EN] REST endpoint literals.
const (
	pathCollectionsHas    = "/collections/has"
	pathCollectionsCreate = "/collections/create"
	pathCollectionsDescr  = "/collections/describe"
	pathCollectionsLoad   = "/collections/load"
	pathEntitiesInsert    = "/entities/insert"
	pathEntitiesFlush     = "/entities/flush"
	pathEntitiesSearch    = "/entities/search"
)

// 集合与字段默认名（langchaingo 同名，存量集合直读）.
// [EN] Default collection/field names (aligned with langchaingo).
const (
	DefaultCollectionName = "LangChainGoCollection"
	DefaultPrimaryField   = "pk"
	DefaultTextField      = "text"
	DefaultMetaField      = "meta"
	DefaultVectorField    = "vector"
)

// 集合 schema 字面量.
// [EN] Collection schema literals.
const (
	dataTypeInt64        = "Int64"
	dataTypeVarChar      = "VarChar"
	dataTypeJSON         = "JSON"
	dataTypeFloatVector  = "FloatVector"
	typeParamDim         = "dim"
	typeParamMaxLength   = "max_length"
	defaultMaxLength      = 65535
	defaultIndexName     = "vector_index"
	defaultVectorIndex   = "AUTOINDEX"
)

// 度量类型.
// [EN] Metric types.
const (
	MetricCosine  = "COSINE"
	MetricIP      = "IP"
	MetricL2      = "L2"
)

// 响应码.
// [EN] Response codes.
const (
	codeSuccess = 0
)

// 检索兜底.
// [EN] Search fallback.
const (
	defaultTopK = 10
)
