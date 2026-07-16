/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-16 19:08:24
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-16 19:08:24
 * @FilePath: \go-llmx\chain\router.go
 * @Description: 路由链 —— LLM 判定意图后转发目的地链，
 * 对齐 langchaingo MultiPromptChain 且结构化输出解析更稳
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"
	"sort"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// routerPromptHead 路由判定模板头（目的地清单注入）.
// [EN] Router template head (destination list injected).
const routerPromptHead = `Given a raw text input to a language model, select the model best suited for the input.
You will be given the names of the available models.

<< FORMATTING >>
Return a string with the name of the model to use, and nothing else. Do not include quotes or any other characters.

<< MODELS >>
%s

<< INPUT >>`

// Router 路由链：意图判定 → 目的地链转发.
// [EN] Router chain: intent detection → destination dispatch.
type Router struct {
	// Model 路由判定模型（可与目的地模型不同）.
	// [EN] Routing model (may differ from destination models).
	Model llmx.Model

	// Destinations 目的地链（键为目的地名）.
	// [EN] Destination chains.
	Destinations map[string]Chain

	// Default 缺省目的地（解析失败/未知目的地的兜底；空串报错）.
	// [EN] Fallback destination ("" = error instead).
	Default string

	// namesCache 排序后的目的地清单（构造期一次，路由提示词稳定）.
	// [EN] Sorted destination names (stable prompt).
	namesCache []string

	// listCache 目的地清单文本.
	// [EN] Destination list text.
	listCache string
}

// NewRouter 构造路由链（目的地清单排序缓存）.
// [EN] Build a router (sorted destination cache).
func NewRouter(model llmx.Model, destinations map[string]Chain) *Router {
	r := &Router{Model: model, Destinations: destinations}
	r.rebuildCache()
	return r
}

// WithDefault 设置缺省目的地.
// [EN] Set the fallback destination.
func (r *Router) WithDefault(name string) *Router {
	r.Default = name
	return r
}

// rebuildCache 重建排序缓存（map 遍历序随机，排序保证提示词稳定可测）.
// [EN] Rebuild the sorted cache (stable prompts).
func (r *Router) rebuildCache() {
	r.namesCache = make([]string, 0, len(r.Destinations))
	for name := range r.Destinations {
		r.namesCache = append(r.namesCache, name)
	}
	sort.Strings(r.namesCache)
	r.listCache = strings.Join(r.namesCache, "\n")
}

// Route 判定目的地并转发.
// [EN] Resolve the destination and dispatch.
func (r *Router) Route(ctx context.Context, input string) (string, error) {
	if r == nil || r.Model == nil || len(r.Destinations) == 0 {
		return "", fmt.Errorf("%w: router requires a model and destinations", llmx.ErrInvalidRequest)
	}

	rendered := fmt.Sprintf(routerPromptHead, r.listCache)
	resp, err := r.Model.GenerateContent(ctx, []llmx.Message{llmx.User(rendered + "\n" + input + "\n<< OUTPUT >>")})
	if err != nil {
		return "", err
	}
	dest, err := llmx.FirstText(resp)
	if err != nil {
		return "", err
	}

	dest = normalizeDestination(dest)
	if dest == "" {
		return "", fmt.Errorf("%w: router model returned an empty destination", llmx.ErrInvalidResponse)
	}
	if _, ok := r.Destinations[dest]; !ok {
		if r.Default == "" {
			return "", fmt.Errorf("%w: unknown router destination %q", llmx.ErrInvalidResponse, dest)
		}
		dest = r.Default
	}
	return r.Destinations[dest].Run(ctx, input)
}

// normalizeDestination 清洗模型返回的目的地名（引号/空白/句点）.
// [EN] Normalize the returned destination name.
func normalizeDestination(dest string) string {
	dest = strings.TrimSpace(dest)
	dest = strings.Trim(dest, "\"'`.,;:")
	return strings.TrimSpace(dest)
}
