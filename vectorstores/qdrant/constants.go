/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-14 21:33:08
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-14 21:33:08
 * @FilePath: \go-llmx\vectorstores\qdrant\constants.go
 * @Description: Qdrant 常量 —— REST 端点、payload 字段名与
 * langchaingo/qdrant 默认对齐（page_content/metadata），存量集合平滑读写
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcqdrant

const (
	// DefaultBaseURL 默认 Qdrant REST 地址.
	// [EN] Default Qdrant REST address.
	DefaultBaseURL = "http://localhost:6333"
)

// REST 资源路径字面量（集合名运行期拼接）.
// [EN] REST resource path literals (collection name joined at runtime).
const (
	pathCollections = "/collections"
	pathPoints      = "/points"
	pathSearch      = "/points/search"
)

// payload 字段名（langchaingo 同名，存量集合直读）.
// [EN] Payload field names (aligned with langchaingo).
const (
	payloadContent = "page_content"
	payloadMetadata = "metadata"
)

// 距离度量（Qdrant 原生字面量）.
// [EN] Distance metrics (Qdrant-native literals).
const (
	DistanceCosine  = "Cosine"
	DistanceEuclid = "Euclid"
	DistanceDot     = "Dot"
)

// 检索兜底.
// [EN] Search fallback.
const (
	defaultTopK = 10
)
