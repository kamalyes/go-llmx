/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 22:58:11
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 22:58:11
 * @FilePath: \go-llmx\tool\calculator_test.go
 * @Description: 计算器工具测试 —— 优先级/右结合幂/一元负号/括号/
 * 除零/非法表达式/工具协议对接，纯函数零依赖
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package tool

import (
	"context"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvalExpression 优先级与结合性.
// [EN] Precedence and associativity.
func TestEvalExpression(t *testing.T) {
	cases := []struct {
		expr string
		want float64
	}{
		{"2+3*4", 14},
		{"(2+3)*4", 20},
		{"2^3^2", 512},      // 右结合
		{"10/4", 2.5},       // 浮点除
		{"7 % 3", 1},        // 取模
		{"-5+3", -2},        // 一元负号
		{"2*-3", -6},        // 一元负号中缀后
		{"-(2+3)", -5},      // 括号前一元负
		{"1.5*2", 3},
		{" 2 + 3 ", 5}, // 空白容忍
	}
	for _, c := range cases {
		got, err := EvalExpression(c.expr)
		require.NoError(t, err, c.expr)
		assert.InDelta(t, c.want, got, 1e-9, c.expr)
	}
}

// TestEvalExpression_Errors 错误路径.
// [EN] Error paths.
func TestEvalExpression_Errors(t *testing.T) {
	for _, expr := range []string{
		"", "2+", "(2+3", "2 3", "a+b", "2..3", ")2(",
	} {
		_, err := EvalExpression(expr)
		assert.Error(t, err, "expr %q should fail", expr)
	}
}

// TestEvalExpression_DivZero 除零.
// [EN] Division by zero.
func TestEvalExpression_DivZero(t *testing.T) {
	_, err := EvalExpression("1/0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zero")
}

// TestCalculator_ToolProtocol 工具协议对接（Schema/Func 调用）.
// [EN] Tool protocol wiring.
func TestCalculator_ToolProtocol(t *testing.T) {
	c := NewCalculator()
	assert.Equal(t, "calculator", c.Name)

	out, err := c.Func(context.Background(), `{"expression":"(1+2)*6"}`)
	require.NoError(t, err)
	assert.Equal(t, "18", out)

	// 非法 JSON 参数 → ErrInvalidRequest
	// [EN] Malformed arguments.
	_, err = c.Func(context.Background(), `{bad`)
	assert.ErrorIs(t, err, llmx.ErrInvalidRequest)
}
