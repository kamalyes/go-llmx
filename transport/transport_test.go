/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-28 21:28:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-28 21:28:00
 * @FilePath: \go-llmx\transport\transport_test.go
 * @Description: 传输层全链路测试 —— SSE 解析协议分支 / HTTP 状态分类 / JSON 请求响应
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errEOF 测试统一终止信号（语义等价 io.EOF）.
// [EN] Unified termination signal in tests (equivalent to io.EOF).
var errEOF = io.EOF

func TestReadSSE_BasicFrames(t *testing.T) {
	stream := "data: {\"a\":1}\n\n" +
		"data: {\"a\":2}\n\n" +
		": heartbeat\n\n" +
		"data: [DONE]\n\n"

	var events []SSEEvent
	err := ReadSSE(strings.NewReader(stream), func(ev SSEEvent) error {
		events = append(events, ev)
		if ev.Data == "[DONE]" {
			return errEOF
		}
		return nil
	})
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, `{"a":1}`, events[0].Data)
	assert.Equal(t, `{"a":2}`, events[1].Data)
	assert.Equal(t, "[DONE]", events[2].Data)
}

func TestReadSSE_EventNameAndMultiLineData(t *testing.T) {
	// event: 行 + 多行 data 拼接（\n 连接）
	stream := "event: message\n" +
		"data: line1\n" +
		"data: line2\n\n"

	var got SSEEvent
	err := ReadSSE(strings.NewReader(stream), func(ev SSEEvent) error {
		got = ev
		return errEOF
	})
	require.NoError(t, err)
	assert.Equal(t, "message", got.Event)
	assert.Equal(t, "line1\nline2", got.Data)
}

func TestReadSSE_CRLFAndNoSpaceAfterColon(t *testing.T) {
	// \r\n 行尾 + "data:"后无空格（协议允许两种形态）
	stream := "data:nospace\r\n\r\n"

	var got SSEEvent
	err := ReadSSE(strings.NewReader(stream), func(ev SSEEvent) error {
		got = ev
		return errEOF
	})
	require.NoError(t, err)
	assert.Equal(t, "nospace", got.Data)
}

func TestReadSSE_ErrorPropagates(t *testing.T) {
	boom := errors.New("boom")
	err := ReadSSE(strings.NewReader("data: x\n\n"), func(SSEEvent) error { return boom })
	assert.ErrorIs(t, err, boom)
}

func TestReadSSE_ChunkedArrival(t *testing.T) {
	// 一次一个字节到达（模拟慢网络下的分片读取）
	stream := "data: abc\n\ndata: def\n\n"
	err := ReadSSE(iotest.OneByteReader(strings.NewReader(stream)), func(ev SSEEvent) error {
		if ev.Data == "def" {
			return errEOF
		}
		return nil
	})
	require.NoError(t, err)
}

func TestIsDoneMarker(t *testing.T) {
	assert.True(t, IsDoneMarker("[DONE]"))
	assert.False(t, IsDoneMarker("[done]"))
	assert.False(t, IsDoneMarker("data"))
}

func TestClassifyStatus(t *testing.T) {
	cases := map[int]HTTPStatusClass{
		200: ClassSuccess, 204: ClassSuccess, 299: ClassSuccess,
		400: ClassInvalidRequest, 422: ClassInvalidRequest,
		401: ClassUnauthorized, 403: ClassUnauthorized,
		404: ClassNotFound,
		429: ClassRateLimited,
		500: ClassServerError, 503: ClassServerError,
		302: ClassUnknown, 418: ClassUnknown,
	}
	for status, want := range cases {
		assert.Equal(t, want, ClassifyStatus(status), "status=%d", status)
	}
}

func TestNewStatusError_BodyTruncation(t *testing.T) {
	long := strings.Repeat("x", MaxErrorBodySnippet+100)
	e := NewStatusError(500, long)
	assert.Len(t, e.Body, MaxErrorBodySnippet)
	assert.Equal(t, ClassServerError, e.Class)
}

func TestClient_DoJSON_Success(t *testing.T) {
	var captured map[string]any
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		gotHeader = r.Header.Get("X-Custom")
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := NewClient()
	var out struct {
		OK bool `json:"ok"`
	}
	err := c.PostJSON(context.Background(), srv.URL, map[string]string{"q": "hi"}, &out,
		map[string]string{"X-Custom": "v1"})
	require.NoError(t, err)
	assert.True(t, out.OK)
	assert.Equal(t, "hi", captured["q"])
	assert.Equal(t, "v1", gotHeader)
}

func TestClient_DoJSON_NilOutSuccess(t *testing.T) {
	// 2xx + out 为 nil：跳过解析直接返回
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ignored":true}`)
	}))
	defer srv.Close()

	c := NewClient()
	require.NoError(t, c.PostJSON(context.Background(), srv.URL, nil, nil, nil))
}

func TestClient_DoStream_HeadersAndNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer t", r.Header.Get("Authorization"))
		fmt.Fprint(w, "data: 1\n\n")
	}))
	defer srv.Close()

	c := NewClient()
	err := c.DoStream(context.Background(), MethodPost, srv.URL, nil,
		map[string]string{"Authorization": "Bearer t"}, func(SSEEvent) error { return errEOF })
	require.NoError(t, err)

	// 网络错误（连接拒绝）
	err = c.DoStream(context.Background(), MethodPost, "http://127.0.0.1:1/x", nil, nil, func(SSEEvent) error { return nil })
	assert.ErrorIs(t, err, ErrNetworkUnavailable)
}

func TestReadSSE_NaturalEnd(t *testing.T) {
	// 流自然读完（无终止标记）→ 正常 nil 返回
	var count int
	err := ReadSSE(strings.NewReader("data: a\n\ndata: b\n\n"), func(SSEEvent) error {
		count++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestClient_DoJSON_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":"rate"}`)
	}))
	defer srv.Close()

	c := NewClient()
	var se *StatusError
	err := c.PostJSON(context.Background(), srv.URL, map[string]string{}, nil, nil)
	require.Error(t, err)
	require.ErrorAs(t, err, &se)
	assert.Equal(t, ClassRateLimited, se.Class)
	assert.Equal(t, 429, se.Status)
}

func TestClient_DoJSON_NetworkError(t *testing.T) {
	c := NewClient()
	// 关闭的端口必然网络错误
	err := c.PostJSON(context.Background(), "http://127.0.0.1:1/nope", map[string]string{}, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNetworkUnavailable)
}

func TestClient_DoStream_SSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentTypeEvent)
		fmt.Fprint(w, "data: {\"delta\":\"a\"}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClient()
	var frames []string
	err := c.DoStream(context.Background(), MethodPost, srv.URL, map[string]string{"stream": "true"}, nil,
		func(ev SSEEvent) error {
			frames = append(frames, ev.Data)
			if IsDoneMarker(ev.Data) {
				return errEOF
			}
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, []string{`{"delta":"a"}`, "[DONE]"}, frames)
}

func TestClient_DoStream_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, "unauthorized")
	}))
	defer srv.Close()

	c := NewClient()
	err := c.DoStream(context.Background(), MethodPost, srv.URL, nil, nil, func(SSEEvent) error { return nil })
	var se *StatusError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, ClassUnauthorized, se.Class)
}

func TestClient_TimeoutOption(t *testing.T) {
	c := NewClient(WithTimeout(0)) // 0 忽略，保持默认
	assert.Equal(t, defaultTimeoutDuration, c.httpClient.Timeout)

	c2 := NewClient(WithTimeout(50 * time.Millisecond))
	assert.Equal(t, 50*time.Millisecond, c2.httpClient.Timeout)
}

func TestClient_HTTPClientOption(t *testing.T) {
	h := &http.Client{Timeout: 7 * time.Second}
	c := NewClient(WithHTTPClient(h))
	assert.Same(t, h, c.httpClient)

	// nil 注入忽略
	c2 := NewClient(WithHTTPClient(nil))
	assert.NotNil(t, c2.httpClient)
}

func TestClient_MarshalRequestError(t *testing.T) {
	// map 含 channel → json.Marshal 必败
	c := NewClient()
	err := c.PostJSON(context.Background(), "http://example.invalid",
		map[string]any{"ch": make(chan int)}, nil, nil)
	assert.ErrorIs(t, err, ErrMarshalRequest)
}

func TestClient_UnmarshalResponseError(t *testing.T) {
	// 2xx 但非法 JSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not-json")
	}))
	defer srv.Close()

	c := NewClient()
	var out map[string]any
	err := c.PostJSON(context.Background(), srv.URL, nil, &out, nil)
	assert.ErrorIs(t, err, ErrUnmarshalResponse)
}

func TestStatusError_ErrorString(t *testing.T) {
	e := NewStatusError(500, "boom")
	assert.Contains(t, e.Error(), "status=500")
	assert.Contains(t, e.Error(), "boom")
}

func TestReadSSE_ReaderError(t *testing.T) {
	// 底层 Reader 报错（非 EOF）→ 原样透传
	err := ReadSSE(iotest.ErrReader(errors.New("net broken")), func(SSEEvent) error { return nil })
	assert.ErrorContains(t, err, "net broken")
}

func TestClient_DoStreamMarshalError(t *testing.T) {
	c := NewClient()
	err := c.DoStream(context.Background(), MethodPost, "http://example.invalid",
		map[string]any{"ch": make(chan int)}, nil, func(SSEEvent) error { return nil })
	assert.ErrorIs(t, err, ErrMarshalRequest)
}

func TestClient_InvalidMethod(t *testing.T) {
	// 含空格的方法名 → http.NewRequest 报错
	c := NewClient()
	err := c.DoJSON(context.Background(), "G ET", "http://example.invalid", nil, nil, nil)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNetworkUnavailable)

	err = c.DoStream(context.Background(), "G ET", "http://example.invalid", nil, nil, func(SSEEvent) error { return nil })
	require.Error(t, err)
}

func TestReadSSE_EmptyEventBoundary(t *testing.T) {
	// 前置空行边界（无 data/event）不触发回调
	stream := "\n\ndata: x\n\n"
	var count int
	err := ReadSSE(strings.NewReader(stream), func(SSEEvent) error {
		count++
		return errEOF
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// ============================================================================
// NDJSON 解析（Ollama 等本地推理协议）
// ============================================================================

func TestReadNDJSON_LinesAndCRLF(t *testing.T) {
	// CRLF 行尾容忍 + 逐行回调
	stream := "{\"a\":1}\r\n\r\n{\"a\":2}\n"
	var lines []string
	err := ReadNDJSON(strings.NewReader(stream), func(line string) error {
		lines = append(lines, line)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{`{"a":1}`, `{"a":2}`}, lines)
}

func TestReadNDJSON_LastLineWithoutNewline(t *testing.T) {
	// 末行无尾随换行符 → 仍作为最后一帧触发回调
	var lines []string
	err := ReadNDJSON(strings.NewReader(`{"a":1}`+"\n"+`{"a":2}`), func(line string) error {
		lines = append(lines, line)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{`{"a":1}`, `{"a":2}`}, lines)
}

func TestReadNDJSON_EmptyStream(t *testing.T) {
	// 空流 / 纯空行流 → 不触发回调，正常返回
	var count int
	err := ReadNDJSON(strings.NewReader("\n\n"), func(string) error {
		count++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestReadNDJSON_HandlerEOFTerminates(t *testing.T) {
	// 回调返回 io.EOF → 正常终止（不视为错误）
	var count int
	err := ReadNDJSON(strings.NewReader("{\"a\":1}\n{\"a\":2}\n"), func(string) error {
		count++
		return errEOF
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestReadNDJSON_ErrorPropagates(t *testing.T) {
	wantErr := errors.New("handler failed")
	err := ReadNDJSON(strings.NewReader("{\"a\":1}\n"), func(string) error {
		return wantErr
	})
	assert.ErrorIs(t, err, wantErr)
}

func TestReadNDJSON_LastLineHandlerEOF(t *testing.T) {
	// 末行回调返回 io.EOF → 正常收口（emitNDJSONLine 分支）
	err := ReadNDJSON(strings.NewReader("{\"a\":1}"), func(string) error {
		return errEOF
	})
	require.NoError(t, err)
}

func TestReadNDJSON_LastLineErrorPropagates(t *testing.T) {
	// 末行（无换行符）回调返回普通错误 → 原样透传
	wantErr := errors.New("last line failed")
	err := ReadNDJSON(strings.NewReader("{\"a\":1}"), func(string) error {
		return wantErr
	})
	assert.ErrorIs(t, err, wantErr)
}

func TestReadNDJSON_ReaderError(t *testing.T) {
	// 底层 Reader 报错（非 EOF）→ 原样透传
	err := ReadNDJSON(iotest.ErrReader(errors.New("net broken")), func(string) error { return nil })
	assert.ErrorContains(t, err, "net broken")
}

func TestClient_DoNDJSON(t *testing.T) {
	var gotAccept, gotStream string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotStream, _ = body["stream"].(string)
		w.Header().Set("Content-Type", ContentTypeNDJSON)
		fmt.Fprint(w, `{"chunk":"a"}`+"\n"+`{"chunk":"done"}`+"\n")
	}))
	defer srv.Close()

	c := NewClient()
	var lines []string
	err := c.DoNDJSON(context.Background(), MethodPost, srv.URL, map[string]any{"stream": "true"}, nil,
		func(line string) error {
			lines = append(lines, line)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, []string{`{"chunk":"a"}`, `{"chunk":"done"}`}, lines)
	// NDJSON Accept 头形态
	assert.Equal(t, ContentTypeNDJSON, gotAccept)
	assert.Equal(t, "true", gotStream)
}

func TestClient_DoNDJSON_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, "not found")
	}))
	defer srv.Close()

	c := NewClient()
	err := c.DoNDJSON(context.Background(), MethodPost, srv.URL, nil, nil, func(string) error { return nil })
	var se *StatusError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, ClassNotFound, se.Class)
}

func TestClient_DoNDJSON_MarshalError(t *testing.T) {
	c := NewClient()
	err := c.DoNDJSON(context.Background(), MethodPost, "http://example.invalid",
		map[string]any{"ch": make(chan int)}, nil, func(string) error { return nil })
	assert.ErrorIs(t, err, ErrMarshalRequest)
}

func TestClient_DoNDJSON_InvalidMethod(t *testing.T) {
	c := NewClient()
	err := c.DoNDJSON(context.Background(), "G ET", "http://example.invalid", nil, nil, func(string) error { return nil })
	require.Error(t, err)
}
