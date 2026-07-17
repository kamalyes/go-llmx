/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 21:37:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-08 20:57:33
 * @FilePath: \go-llmx\vectorstores\memory\constants.go
 * @Description: 内存向量库常量 —— 检索与存储行为默认值
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package lcmemory

// 检索与存储默认值.
// [EN] Retrieval and storage defaults.
const (
	// DefaultTopK 缺省返回条数（未显式指定 topK 时兜底）.
	// [EN] Default result count (fallback when topK is unspecified).
	DefaultTopK = 4

	// MinVectorDim 最小合法向量维度（空向量无法计算相似度）.
	// [EN] Minimum valid vector dimension.
	MinVectorDim = 1
)

// 并发分片阈值（检索路径）.
// [EN] Sharding thresholds for the search path.
const (
	// ShardThreshold 低于此条数走单线程路径（goroutine 开销反噬临界点）.
	// [EN] Below this count the serial path wins.
	ShardThreshold = 4096

	// ShardMaxWorkers 分片 worker 上限（防 GOMAXPROCS 过大反而抖动）.
	// [EN] Worker cap.
	ShardMaxWorkers = 16
)
