/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 23:29:52
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 23:38:26
 * @FilePath: \go-llmx\adapters\googleai\compliance_test.go
 * @Description: Google AI 适配器合规测试 —— compliance.Suite 标准用例接入.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcgoogleai

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kamalyes/go-llmx/compliance"
)

// complianceMockServer 按路径分发的合规端点（generateContent / streamGenerateContent）.
// [EN] Compliance endpoint dispatching by path.
func complianceMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, methodStreamContent) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, `data: {"candidates":[{"content":{"parts":[{"text":"Hi"}]}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"candidates":[{"content":{"parts":[{"text":" there"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`+"\n\n")
			return
		}
		fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"Hi there"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`)
	}))
}

func TestCompliance(t *testing.T) {
	srv := complianceMockServer(t)
	c := New("test-key", WithBaseURL(srv.URL), WithModel(DefaultModel))

	compliance.New(t, c).Run()
}
