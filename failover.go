/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 09:52:13
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 09:52:13
 * @FilePath: \go-llmx\failover.go
 * @Description: 失败转移与路由模型 —— 依序故障切换 / 随机 / 轮询多策略，
 * 实现 Model 接口即无缝接入 chain / agent / memory 全生态
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
)

// ErrNoModels 路由模型未配置任何后端.
// [EN] Router model has no backends configured.
var ErrNoModels = errors.New("llmx: no models configured")

// failoverOf 判定错误是否值得切换后端（网络不可达 / 服务端错误 / 空响应；
// 参数错误与认证失败换后端也救不了，直接透传）.
// [EN] Whether the error warrants failing over (unavailable / server error /
// empty response; invalid requests and auth errors propagate as-is).
func failoverOf(err error) bool {
	return errors.Is(err, ErrProviderUnavailable) ||
		errors.Is(err, ErrAPIServerError) ||
		errors.Is(err, ErrEmptyResponse)
}

// Failover 依序故障切换模型：主模型失败（不可达 / 5xx / 空响应）时
// 自动切换到下一个后端重试；流式调用在首个 chunk 前失败同样切换.
// [EN] Sequential failover model: on failure (unreachable / 5xx / empty),
// retry with the next backend; streaming fails over before the first chunk.
type Failover struct {
	// models 依序尝试的后端列表（至少一个）.
	// [EN] Backends tried in order (at least one).
	models []Model
}

// NewFailover 构造依序故障切换模型.
// [EN] Build a sequential failover model.
func NewFailover(models ...Model) *Failover {
	return &Failover{models: models}
}

// GenerateContent 实现 Model（依序尝试后端直至成功或耗尽）.
// [EN] Implement Model (try backends in order until success or exhaustion).
func (f *Failover) GenerateContent(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	if len(f.models) == 0 {
		return nil, ErrNoModels
	}
	var lastErr error
	for _, m := range f.models {
		resp, err := m.GenerateContent(ctx, messages, opts...)
		if err == nil {
			return resp, nil
		}
		if !failoverOf(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// StreamGenerateContent 实现 Model（流式切换语义：首个 chunk 到达后不再切换，
// 后端中断直接透传错误——已收内容不可回放，避免重复输出）.
// [EN] Implement Model (streaming fails over only before the first chunk;
// once chunks flow, errors propagate as-is to avoid duplicated output).
func (f *Failover) StreamGenerateContent(ctx context.Context, messages []Message, stream StreamHandler, opts ...Option) (*Response, error) {
	if len(f.models) == 0 {
		return nil, ErrNoModels
	}
	var lastErr error
	for _, m := range f.models {
		// 首 chunk 前失败可切换：包装 handler 记录是否已有产出
		// [EN] Failover only pre-first-chunk: wrap handler to track output.
		started := false
		wrapped := stream
		if stream != nil {
			wrapped = func(chunk *Chunk) error {
				started = true
				return stream(chunk)
			}
		}
		resp, err := m.StreamGenerateContent(ctx, messages, wrapped, opts...)
		if err == nil {
			return resp, nil
		}
		if started || !failoverOf(err) {
			return resp, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// RandomModel 随机路由模型：每次调用随机选择后端（多 Key 分流 / 灰度采样）.
// [EN] Random router: picks a random backend per call (key sharding / sampling).
type RandomModel struct {
	// models 候选后端.
	// [EN] Candidate backends.
	models []Model
}

// NewRandomModel 构造随机路由模型.
// [EN] Build a random router.
func NewRandomModel(models ...Model) *RandomModel {
	return &RandomModel{models: models}
}

// GenerateContent 实现 Model.
// [EN] Implement Model.
func (r *RandomModel) GenerateContent(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	m, err := r.pick()
	if err != nil {
		return nil, err
	}
	return m.GenerateContent(ctx, messages, opts...)
}

// StreamGenerateContent 实现 Model.
// [EN] Implement Model.
func (r *RandomModel) StreamGenerateContent(ctx context.Context, messages []Message, stream StreamHandler, opts ...Option) (*Response, error) {
	m, err := r.pick()
	if err != nil {
		return nil, err
	}
	return m.StreamGenerateContent(ctx, messages, stream, opts...)
}

// pick 随机选择后端.
// [EN] Pick a random backend.
func (r *RandomModel) pick() (Model, error) {
	if len(r.models) == 0 {
		return nil, ErrNoModels
	}
	return r.models[rand.Intn(len(r.models))], nil
}

// RoundRobinModel 轮询路由模型：按调用次数依次分发（均衡配额 / A/B 对照）.
// [EN] Round-robin router: distributes calls in turn (quota balance / A/B).
type RoundRobinModel struct {
	// models 候选后端.
	// [EN] Candidate backends.
	models []Model

	// next 原子游标（并发安全）.
	// [EN] Atomic cursor (concurrency-safe).
	next atomic.Uint32
}

// NewRoundRobinModel 构造轮询路由模型.
// [EN] Build a round-robin router.
func NewRoundRobinModel(models ...Model) *RoundRobinModel {
	return &RoundRobinModel{models: models}
}

// GenerateContent 实现 Model（游标取模轮询）.
// [EN] Implement Model (modulo cursor rotation).
func (r *RoundRobinModel) GenerateContent(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	m, err := r.pick()
	if err != nil {
		return nil, err
	}
	return m.GenerateContent(ctx, messages, opts...)
}

// StreamGenerateContent 实现 Model.
// [EN] Implement Model.
func (r *RoundRobinModel) StreamGenerateContent(ctx context.Context, messages []Message, stream StreamHandler, opts ...Option) (*Response, error) {
	m, err := r.pick()
	if err != nil {
		return nil, err
	}
	return m.StreamGenerateContent(ctx, messages, stream, opts...)
}

// pick 轮询选择后端（无锁原子递增取模）.
// [EN] Pick next backend (lock-free atomic increment modulo).
func (r *RoundRobinModel) pick() (Model, error) {
	if len(r.models) == 0 {
		return nil, ErrNoModels
	}
	i := r.next.Add(1) - 1
	return r.models[int(i%uint32(len(r.models)))], nil
}

// 编译期断言：路由组件实现 Model 契约.
// [EN] Compile-time assertions.
var (
	_ Model = (*Failover)(nil)
	_ Model = (*RandomModel)(nil)
	_ Model = (*RoundRobinModel)(nil)
)

// 未使用 sync 的显式引用（结构体字段保留 mu 以备并发扩展）.
// [EN] Reserved mutex import guard.
var _ = sync.Mutex{}
