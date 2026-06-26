/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-26 22:22:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-26 22:31:57
 * @FilePath: \go-llmx\adapters\ollama\compliance_test.go
 * @Description: Ollama 适配器合规测试 —— compliance.Suite 标准用例接入.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcollama

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
			w.Header().Set("Content-Type", "application/x-ndjson")
			fmt.Fprint(w, `{"message":{"role":"assistant","content":"Hi"},"done":false}`+"\n")
			fmt.Fprint(w, `{"message":{"role":"assistant","content":" there"},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":2}`+"\n")
			return
		}
		fmt.Fprint(w, `{"message":{"role":"assistant","content":"Hi there"},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":2}`)
	}))
}

func TestCompliance(t *testing.T) {
	srv := complianceMockServer(t)
	c := New("test-key", WithBaseURL(srv.URL), WithModel("llama3.2"))

	compliance.New(t, c).Run()
}
