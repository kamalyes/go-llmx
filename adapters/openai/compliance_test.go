/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-26 22:23:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-26 22:31:57
 * @FilePath: \go-llmx\adapters\openai\compliance_test.go
 * @Description: OpenAI 适配器合规测试 —— compliance.Suite 标准用例接入.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcopenai

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kamalyes/go-llmx/compliance"
)

// complianceMockServer 按 stream 开关分发的合规端点.
// [EN] Compliance endpoint dispatching by the stream flag.
func complianceMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readBodyString(r)
		if strings.Contains(body, `"stream":true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"Hi"}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":" there"}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Hi there"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`)
	}))
}

// readBodyString 读取请求体为字符串.
// [EN] Read the request body as a string.
func readBodyString(r *http.Request) string {
	buf := make([]byte, 4096)
	n, _ := r.Body.Read(buf)
	return string(buf[:n])
}

func TestCompliance(t *testing.T) {
	srv := complianceMockServer(t)
	c := New("test-key", WithBaseURL(srv.URL), WithModel("gpt-4o-mini"))

	compliance.New(t, c).Run()
}
