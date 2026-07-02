/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 21:31:05
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 21:33:58
 * @FilePath: \go-llmx\adapters\mistral\compliance_test.go
 * @Description: Mistral 适配器合规测试 —— compliance.Suite 标准用例接入.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcmistral

import (
	"fmt"
	"io"
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
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"stream":true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"Hi"}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":" there"}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"Hi there"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`)
	}))
}

func TestCompliance(t *testing.T) {
	srv := complianceMockServer(t)
	c := New("test-key", WithBaseURL(srv.URL), WithModel(DefaultModel))

	compliance.New(t, c).Run()
}
