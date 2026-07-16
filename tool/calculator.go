/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-16 22:38:17
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-16 22:38:17
 * @FilePath: \go-llmx\tool\calculator.go
 * @Description: 计算器工具 —— 表达式求值（零依赖递归下降解析），
 * 替代 langchaingo 依赖 govaluate 的 Calculator
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	llmx "github.com/kamalyes/go-llmx"
)

// calculatorArgs 计算器参数 Schema.
// [EN] Calculator argument schema.
type calculatorArgs struct {
	Expression string `json:"expression"`
}

// NewCalculator 计算器工具：四则运算/幂/括号/一元正负，
// 递归下降求值器零依赖（无 CGO、无第三方表达式引擎）.
// [EN] Calculator tool: arithmetic/power/parens/unary sign.
func NewCalculator() Tool {
	return Tool{
		Name:        "calculator",
		Description: "Useful for math expressions like (2+3)*4 or 2^10. Input a plain expression.",
		Parameters:  SchemaOf(calculatorArgs{}),
		Func: func(ctx context.Context, arguments string) (string, error) {
			args, err := unmarshalToolArgs[calculatorArgs](arguments)
			if err != nil {
				return "", err
			}
			val, err := EvalExpression(args.Expression)
			if err != nil {
				return "", err
			}
			return strconv.FormatFloat(val, 'g', -1, 64), nil
		},
	}
}

// unmarshalToolArgs 工具参数反序列化（arguments → 结构体）.
// [EN] Unmarshal tool arguments.
func unmarshalToolArgs[T any](arguments string) (T, error) {
	var args T
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return args, fmt.Errorf("%w: %v", llmx.ErrInvalidRequest, err)
	}
	return args, nil
}

// EvalExpression 求值算术表达式：+ - * / % ^ 与括号、一元正负.
// [EN] Evaluate an arithmetic expression.
func EvalExpression(expr string) (float64, error) {
	p := &exprParser{src: strings.TrimSpace(expr)}
	if p.src == "" {
		return 0, fmt.Errorf("empty expression")
	}
	val, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos != len(p.src) {
		return 0, fmt.Errorf("unexpected %q at %d", p.src[p.pos], p.pos)
	}
	return val, nil
}

// exprParser 递归下降解析器.
// [EN] Recursive-descent parser.
type exprParser struct {
	src string
	pos int
}

// parseExpr 加减层（最低优先级）.
// [EN] Additive level.
func (p *exprParser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		op := p.peekOp("+-")
		if op == 0 {
			return left, nil
		}
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
}

// parseTerm 乘除模层.
// [EN] Multiplicative level.
func (p *exprParser) parseTerm() (float64, error) {
	left, err := p.parsePower()
	if err != nil {
		return 0, err
	}
	for {
		op := p.peekOp("*/%")
		if op == 0 {
			return left, nil
		}
		right, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		switch op {
		case '*':
			left *= right
		case '/':
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		case '%':
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			left = math.Mod(left, right)
		}
	}
}

// parsePower 幂层（右结合）.
// [EN] Power level (right-associative).
func (p *exprParser) parsePower() (float64, error) {
	base, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	if p.peekOp("^") == '^' {
		exp, err := p.parsePower() // 右结合：2^3^2 = 2^(3^2)
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

// parseUnary 一元正负与原子.
// [EN] Unary sign and atoms.
func (p *exprParser) parseUnary() (float64, error) {
	p.skipSpace()
	if p.peekOp("-+") != 0 {
		op := p.src[p.pos]
		p.pos++
		val, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		if op == '-' {
			return -val, nil
		}
		return val, nil
	}
	return p.parseAtom()
}

// parseAtom 数字与括号.
// [EN] Numbers and parentheses.
func (p *exprParser) parseAtom() (float64, error) {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return 0, fmt.Errorf("unexpected end of expression")
	}
	if p.src[p.pos] == '(' {
		p.pos++
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.pos >= len(p.src) || p.src[p.pos] != ')' {
			return 0, fmt.Errorf("missing closing paren")
		}
		p.pos++
		return val, nil
	}
	start := p.pos
	for p.pos < len(p.src) && (unicode.IsDigit(rune(p.src[p.pos])) || p.src[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("unexpected %q at %d", p.src[p.pos], p.pos)
	}
	return strconv.ParseFloat(p.src[start:p.pos], 64)
}

// peekOp 窗口内查看运算符（匹配则消费）.
// [EN] Peek and consume an operator.
func (p *exprParser) peekOp(ops string) byte {
	p.skipSpace()
	if p.pos < len(p.src) {
		if c := p.src[p.pos]; strings.IndexByte(ops, c) >= 0 {
			p.pos++
			return c
		}
	}
	return 0
}

// skipSpace 跳过空白.
// [EN] Skip whitespace.
func (p *exprParser) skipSpace() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
}
