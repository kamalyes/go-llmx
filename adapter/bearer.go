/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-17 11:02:36
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-17 11:02:36
 * @FilePath: \go-llmx\adapter\bearer.go
 * @Description: Bearer 认证头公共件 —— OpenAI/Mistral/Cohere/Ollama 等
 * 通行形态统一收口（空密钥不发头 + 密钥轮换检测 + 共享只读缓存零分配）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package adapter

// BearerHeaders Bearer 形态认证头（Authorization: Bearer <key>）.
// [EN] Bearer-style auth headers.
//
// 空密钥返回空 map（本地部署/无认证网关不发无效头）；
// 密钥未轮换时返回共享只读 map（零分配；transport 层只遍历不修改）；
// SetAPIKey 轮换后下次调用自动失效重建
func (b *Base) BearerHeaders() map[string]string {
	b.bearer.mu.Lock()
	defer b.bearer.mu.Unlock()
	if b.bearer.headers != nil && b.bearer.key == b.APIKey {
		return b.bearer.headers
	}
	h := map[string]string{}
	if b.APIKey != "" {
		h["Authorization"] = "Bearer " + b.APIKey
	}
	b.bearer.key = b.APIKey
	b.bearer.headers = h
	return h
}
