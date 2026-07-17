/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-17 11:02:36
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-17 11:02:36
 * @FilePath: \go-llmx\adapter\embed.go
 * @Description: Embedder 公共件 —— 各嵌入适配器重复的空校验与
 * 向量数不匹配错误统一收口（编解码等协议差异仍留在各适配器）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package adapter

import (
	"fmt"

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
