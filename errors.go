/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-15 21:26:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-15 21:26:00
 * @FilePath: \go-llmx\errors.go
 * @Description: 哨兵错误定义 —— 全部可 errors.Is 判定，适配器负责将 provider
 * 特定错误映射到本包错误（对齐 go-dbsync 错误风格）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package llmx

import "errors"

var (
	// ErrEmptyResponse 模型返回空响应（无任何 choice）.
	// [EN] Model returned an empty response with no choices.
	ErrEmptyResponse = errors.New("llmx: empty response from model")

	// ErrProviderUnavailable provider 服务不可达（网络错误 / 5xx）.
	// [EN] Provider service is unreachable (network error / 5xx).
	ErrProviderUnavailable = errors.New("llmx: provider unavailable")

	// ErrUnauthorized 认证失败（API Key 无效 / 过期）.
	// [EN] Authentication failed (invalid or expired API key).
	ErrUnauthorized = errors.New("llmx: unauthorized")

	// ErrRateLimited 触发 provider 限流（429）.
	// [EN] Provider rate limit exceeded (429).
	ErrRateLimited = errors.New("llmx: rate limited")

	// ErrInvalidRequest 请求参数非法（400 / 422）.
	// [EN] Invalid request parameters (400 / 422).
	ErrInvalidRequest = errors.New("llmx: invalid request")

	// ErrStreamClosed 流式回调已主动终止（StreamHandler 返回 ErrStopStream）.
	// [EN] Stream terminated by callback returning ErrStopStream.
	ErrStreamClosed = errors.New("llmx: stream closed by handler")

	// ErrStopStream StreamHandler 返回本哨兵可提前终止流式生成（正常语义，非错误）.
	// [EN] Return this sentinel from StreamHandler to stop streaming early (not an error).
	ErrStopStream = errors.New("llmx: stop stream")

	// ErrToolNotFound 工具调用循环中模型请求了未注册的工具.
	// [EN] Model requested a tool that is not registered in the loop.
	ErrToolNotFound = errors.New("llmx: tool not found")

	// ErrMaxToolIterations 工具调用循环达到最大迭代次数仍未产出最终回答.
	// [EN] Tool loop reached max iterations without a final answer.
	ErrMaxToolIterations = errors.New("llmx: max tool iterations exceeded")

	// ErrUnsupportedOperation 适配器不支持该能力（如 provider 不支持图片输入）.
	// [EN] Adapter does not support this capability.
	ErrUnsupportedOperation = errors.New("llmx: unsupported operation")

	// ErrAPIServerError provider 服务端内部错误.
	// [EN] Provider server internal error.
	ErrAPIServerError = errors.New("llmx: api server error")

	// ErrInvalidVectors 向量载荷非法（数量不匹配 / 维度为零）.
	// [EN] Invalid vector payload (count mismatch / zero dimension).
	ErrInvalidVectors = errors.New("llmx: invalid vectors")
)
