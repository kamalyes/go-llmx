/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-17 11:02:36
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-18 09:05:33
 * @FilePath: \go-llmx\adapter\embed.go
 * @Description: Embedder 公共件 —— 空校验/向量数不匹配错误/批量并行派发
 * 统一收口（编解码等协议差异仍留在各适配器）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package adapter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	llmx "github.com/kamalyes/go-llmx"
)

// ValidateEmbedTexts 嵌入输入空校验（EmbedDocuments 前置拦截）.
// [EN] Validate embed inputs (guard for EmbedDocuments).
func ValidateEmbedTexts(texts []string) error {
	if len(texts) == 0 {
		return fmt.Errorf("%w: no texts to embed", llmx.ErrInvalidRequest)
	}
	return nil
}

// ErrVectorCountMismatch 响应向量数与输入文本数不匹配.
// [EN] Response vector count does not match the input texts.
func ErrVectorCountMismatch(got, want int) error {
	return fmt.Errorf("%w: %d vectors for %d texts", llmx.ErrEmptyResponse, got, want)
}

// EmbedBatchWorkers 嵌入批量派发的默认并发（多网关限流友好的保守值）.
// [EN] Default batch concurrency for embedding dispatch (gateway-friendly).
const EmbedBatchWorkers = 4

// ParallelBatches 将 total 条输入按 maxBatch 切批派发 fn（[start,end) 左闭右开），
// 有界并行 + 首错取消：任一批失败返回首个错误，其余批次随派生 ctx 中止收尾
// （预取消的 ctx 仍派发首批，失败语义由 fn 内部收口——不劫持传输层取消日志）.
// [EN] Dispatch fn over [0,total) in maxBatch slices with bounded parallelism
// and fail-fast: the first batch error is returned and the rest are aborted
// via a derived context (a pre-cancelled ctx still dispatches the first
// batch, preserving transport-layer cancellation semantics).
//
// 单批（maxBatch >= total）直调零开销；workers <= 1 串行；
// fn 收到派生 ctx（取消仅影响本组批次，不波及调用方 ctx）
func ParallelBatches(ctx context.Context, total, maxBatch, workers int, fn func(ctx context.Context, start, end int) error) error {
	if total <= 0 {
		return nil
	}
	if maxBatch <= 0 || maxBatch >= total {
		return fn(ctx, 0, total)
	}
	if workers <= 1 {
		for start := 0; start < total; start += maxBatch {
			end := start + maxBatch
			if end > total {
				end = total
			}
			if err := fn(ctx, start, end); err != nil {
				return err
			}
		}
		return nil
	}

	batches := (total + maxBatch - 1) / maxBatch
	if workers > batches {
		workers = batches
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		first  error
		cursor atomic.Int64
	)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for {
				idx := int(cursor.Add(1) - 1)
				start := idx * maxBatch
				if start >= total {
					return
				}
				end := start + maxBatch
				if end > total {
					end = total
				}
				if err := fn(ctx, start, end); err != nil {
					mu.Lock()
					if first == nil {
						first = err
					}
					mu.Unlock()
					cancel()
					return
				}
				if ctx.Err() != nil {
					return
				}
			}
		}()
	}
	wg.Wait()
	if first != nil {
		return first
	}
	// 外部取消且未全部完成时如实返回取消错误
	return ctx.Err()
}
