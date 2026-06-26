/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-26 22:26:10
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-26 22:31:57
 * @FilePath: \go-llmx\adapters\anthropic\compliance_test.go
 * @Description: Anthropic 适配器合规测试 —— compliance.Suite 标准用例接入.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcanthropic

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
			fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
			fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n")
			fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" there\"}}\n\n")
			fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n")
			fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
			return
		}
		fmt.Fprint(w, `{"content":[{"type":"text","text":"Hi there"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`)
	}))
}

func TestCompliance(t *testing.T) {
	srv := complianceMockServer(t)
	c := New("test-key", WithBaseURL(srv.URL), WithModel("claude-sonnet-4-5"))

	compliance.New(t, c).Run()
}
