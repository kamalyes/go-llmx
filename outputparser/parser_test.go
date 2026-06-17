/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-17 21:02:26
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-17 21:08:37
 * @FilePath: \go-llmx\outputparser\parser_test.go
 * @Description: 解析器契约测试 —— ParseError 哨兵判定与错误消息形态.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package outputparser

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseError_Sentinel(t *testing.T) {
	err := ParseError{Text: "raw output", Reason: "no ```json block in output"}

	require.ErrorIs(t, err, ErrParse)
	assert.Contains(t, err.Error(), "raw output")
	assert.Contains(t, err.Error(), "no ```json block in output")
}

func TestParseError_ErrorsIs(t *testing.T) {
	wrapped := errors.Join(ErrParse)

	assert.ErrorIs(t, wrapped, ErrParse)
	assert.NotErrorIs(t, assert.AnError, ErrParse)
}
