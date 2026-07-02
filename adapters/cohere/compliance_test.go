/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-02 23:25:41
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-02 23:38:26
 * @FilePath: \go-llmx\adapters\cohere\compliance_test.go
 * @Description: Cohere 适配器合规测试 —— compliance.Suite 标准用例接入.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lccohere

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
			fmt.Fprint(w, `data: {"type":"content-start","index":0}`+"\n\n")
			fmt.Fprint(w, `data: {"type":"content-delta","index":0,"delta":{"message":{"content":{"text":"Hi"}}}}`+"\n\n")
			fmt.Fprint(w, `data: {"type":"content-delta","index":0,"delta":{"message":{"content":{"text":" there"}}}}`+"\n\n")
			fmt.Fprint(w, `data: {"type":"content-end","index":0}`+"\n\n")
			fmt.Fprint(w, `data: {"type":"message-end","delta":{"finish_reason":"COMPLETE","usage":{"tokens":{"input_tokens":5,"output_tokens":2}}}}`+"\n\n")
			return
		}
		fmt.Fprint(w, `{"id":"r1","message":{"role":"assistant","content":[{"type":"text","text":"Hi there"}]},"finish_reason":"COMPLETE","usage":{"tokens":{"input_tokens":5,"output_tokens":2}}}`)
	}))
}

func TestCompliance(t *testing.T) {
	srv := complianceMockServer(t)
	c := New("test-key", WithBaseURL(srv.URL), WithModel(DefaultModel))

	compliance.New(t, c).Run()
}
