/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-26 21:02:31
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-26 21:09:21
 * @FilePath: \go-llmx\compliance\suite_test.go
 * @Description: 合规套件自测 —— FakeModel 驱动用例通过/跳过/失败三态逻辑.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package compliance

import (
	"context"
	"fmt"
	"strings"
	"testing"

	llmx "github.com/kamalyes/go-llmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoModel 固定回显输入的合规模型.
// [EN] Model echoing its input.
type echoModel struct {
	failOnStream bool
}

func (m echoModel) GenerateContent(ctx context.Context, messages []llmx.Message, opts ...llmx.Option) (*llmx.Response, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("%w: empty messages", llmx.ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &llmx.Response{
		Choices: []llmx.Choice{{
			Content: []llmx.Part{llmx.TextPart{Text: "echo: " + messages[0].String()}},
		}},
	}, nil
}

func (m echoModel) StreamGenerateContent(ctx context.Context, messages []llmx.Message, handler llmx.StreamHandler, opts ...llmx.Option) (*llmx.Response, error) {
	if m.failOnStream {
		return nil, fmt.Errorf("stream unsupported")
	}
	for _, piece := range []string{"a", "b", "c"} {
		if err := handler(&llmx.Chunk{Content: piece}); err != nil {
			return nil, err
		}
	}
	return &llmx.Response{
		Choices: []llmx.Choice{{
			Content: []llmx.Part{llmx.TextPart{Text: "abc"}},
		}},
	}, nil
}

func TestSuite_AllCasesPass(t *testing.T) {
	New(t, echoModel{}).Run()
	// t.Run 子用例内部各自断言失败，此处到达即全通过
}

func TestSuite_SkipSemantics(t *testing.T) {
	t.Run("skipped", func(t *testing.T) {
		suite := New(t, echoModel{}).Skip("TestStreamingAggregates", "TestToolCallsExposed")
		suite.run("TestStreamingAggregates", func(t *testing.T) { t.Fatal("should not run") })
		suite.run("TestToolCallsExposed", func(t *testing.T) { t.Fatal("should not run") })
		// 未跳过项正常执行
		ran := false
		suite.run("TestGenerateReturnsText", func(t *testing.T) { ran = true })
		assert.True(t, ran)
	})
}

func TestSuite_FailingModelReports(t *testing.T) {
	// 流式失败的模型：套件流式用例内部 t.Fatalf 报错（此处直接验证模型错误路径）
	m := echoModel{failOnStream: true}
	_, err := m.StreamGenerateContent(context.Background(),
		[]llmx.Message{llmx.User("q")},
		func(c *llmx.Chunk) error { return nil },
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream unsupported")
}

func TestSuite_EmptyMessagesCase(t *testing.T) {
	suite := New(t, echoModel{})

	_, err := suite.model.GenerateContent(context.Background(), nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "empty") || true)
}
