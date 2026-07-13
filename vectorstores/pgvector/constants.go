/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-13 21:07:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-13 21:07:33
 * @FilePath: \go-llmx\vectorstores\pgvector\constants.go
 * @Description: pgvector 常量 —— 表名/集合名与 langchaingo/pgvector 默认对齐，
 * 存量库零改造平滑读写
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcpgvector

const (
	// DefaultCollectionName 默认集合名（langchaingo 同名，存量集合直读）.
	// [EN] Default collection name (aligned with langchaingo).
	DefaultCollectionName = "langchain"

	// DefaultEmbeddingTable 默认嵌入表名（langchaingo 同名）.
	// [EN] Default embedding table name.
	DefaultEmbeddingTable = "langchain_pg_embedding"

	// DefaultCollectionTable 默认集合表名（langchaingo 同名）.
	// [EN] Default collection table name.
	DefaultCollectionTable = "langchain_pg_collection"

	// DefaultTopK topK 兜底值.
	// [EN] topK fallback.
	DefaultTopK = 10
)

// 距离函数（HNSW 索引算子类）.
// [EN] Distance functions (HNSW operator classes).
const (
	// DistanceCosine 余弦距离（默认，检索语义 <=>）.
	// [EN] Cosine distance (default, searched via <=>).
	DistanceCosine = "vector_cosine_ops"

	// DistanceL2 欧氏距离（检索语义 <->）.
	// [EN] L2 distance (searched via <->).
	DistanceL2 = "vector_l2_ops"

	// DistanceIP 内积（检索语义 <#>）.
	// [EN] Inner product (searched via <#>).
	DistanceIP = "vector_ip_ops"
)
