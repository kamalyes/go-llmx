/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-10-13 20:39:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-10-13 20:39:00
 * @FilePath: \go-llmx\chain\sequential.go
 * @Description: SequentialChain —— 顺序链（前一链输出作为下一链输入），
 * 替代 langchaingo SequentialChain
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package chain

import (
	"context"
	"fmt"

	llmx "github.com/kamalyes/go-llmx"
)

// SequentialChain 顺序链（前一链输出作为下一链输入）.
// [EN] Sequential chain (each chain's output feeds the next).
type SequentialChain struct {
	// chains 子链列表（按序执行）.
	// [EN] Child chains (executed in order).
	chains []Chain
}

// NewSequentialChain 构造顺序链.
// [EN] Build a sequential chain.
func NewSequentialChain(chains ...Chain) *SequentialChain {
	return &SequentialChain{chains: chains}
}

// Run 实现 Chain（空链原样返回输入）.
// [EN] Implement Chain (an empty chain returns the input as-is).
func (s *SequentialChain) Run(ctx context.Context, input string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("%w: SequentialChain is nil", llmx.ErrInvalidRequest)
	}
	for _, c := range s.chains {
		if c == nil {
			return "", fmt.Errorf("%w: sequential chain contains nil", llmx.ErrInvalidRequest)
		}
		out, err := c.Run(ctx, input)
		if err != nil {
			return "", err
		}
		input = out
	}
	return input, nil
}
