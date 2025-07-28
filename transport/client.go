/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-28 20:39:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-28 20:39:00
 * @FilePath: \go-llmx\transport\client.go
 * @Description: HTTP 传输客户端 —— JSON 请求/响应 + SSE 流式请求的公共收口.
 * 适配器内嵌本客户端：只做协议编解码，传输/超时/错误分类全部收口在 transport 包
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultTimeoutDuration 默认请求超时（LLM 生成普遍慢于常规 API）.
// [EN] Default request timeout (LLM generation is slower than typical APIs).
const defaultTimeoutDuration = 120 * time.Second

// Client JSON-over-HTTP 客户端（适配器内嵌复用）.
// [EN] JSON-over-HTTP client (embedded by adapters).
type Client struct {
	httpClient *http.Client
}

// clientConfig 构造配置（选项作用目标）.
// [EN] Constructor configuration (target of options).
type clientConfig struct {
	httpClient *http.Client
}

// Option 客户端构造选项.
// [EN] Client constructor option.
type Option func(*clientConfig)

// WithTimeout 设置总超时.
// [EN] Set the total timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) {
		if d > 0 {
			c.httpClient = &http.Client{Timeout: d}
		}
	}
}

// WithHTTPClient 注入自定义 http.Client（代理/连接池调优）.
// [EN] Inject a custom http.Client (proxy / pool tuning).
func WithHTTPClient(h *http.Client) Option {
	return func(c *clientConfig) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// NewClient 构造客户端（缺省 120s 超时）.
// [EN] Build a client (120s timeout by default).
func NewClient(opts ...Option) *Client {
	cfg := &clientConfig{
		httpClient: &http.Client{Timeout: defaultTimeoutDuration},
	}
	for _, fn := range opts {
		if fn != nil {
			fn(cfg)
		}
	}
	return &Client{httpClient: cfg.httpClient}
}

// DoJSON 发送 JSON 请求并解析 JSON 响应.
// [EN] Send a JSON request and decode the JSON response.
//
// out 为 nil 时跳过解析只做错误映射；非 2xx 返回 *StatusError（Class 字段供分类判定）
func (c *Client) DoJSON(ctx context.Context, method, url string, in, out any, headers map[string]string) error {
	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return NewRequestMarshalError(err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ContentTypeJSON)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", networkSentinel, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return NewStatusError(resp.StatusCode, string(raw))
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return NewResponseUnmarshalError(err)
	}
	return nil
}

// DoStream 发送 JSON 请求并以 SSE 流回调逐事件消费.
// [EN] Send a JSON request and consume the SSE stream event by event.
//
// on 返回 io.EOF 正常终止流；其它 error 透传中断
func (c *Client) DoStream(ctx context.Context, method, url string, in any, headers map[string]string, on func(ev SSEEvent) error) error {
	body, err := c.sendStreamRequest(ctx, method, url, ContentTypeEvent, in, headers)
	if err != nil {
		return err
	}
	defer body.Close()
	return ReadSSE(body, on)
}

// DoNDJSON 发送 JSON 请求并以 NDJSON 流回调逐行消费（Ollama 等本地推理协议）.
// [EN] Send a JSON request and consume the NDJSON stream line by line.
//
// 终止语义与 DoStream 一致：on 返回 io.EOF 正常终止；其它 error 透传中断
func (c *Client) DoNDJSON(ctx context.Context, method, url string, in any, headers map[string]string, on func(line string) error) error {
	body, err := c.sendStreamRequest(ctx, method, url, ContentTypeNDJSON, in, headers)
	if err != nil {
		return err
	}
	defer body.Close()
	return ReadNDJSON(body, on)
}

// sendStreamRequest 流式请求公共骨架（请求组装/发送/状态检查，响应体由调用方关闭）.
// [EN] Shared streaming request skeleton (assembly/send/status check; caller closes the body).
func (c *Client) sendStreamRequest(ctx context.Context, method, url, accept string, in any, headers map[string]string) (io.ReadCloser, error) {
	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return nil, NewRequestMarshalError(err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", ContentTypeJSON)
	req.Header.Set("Accept", accept)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", networkSentinel, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		return nil, NewStatusError(resp.StatusCode, string(raw))
	}
	return resp.Body, nil
}

// PostJSON 便捷 POST（DoJSON 的方法固定形态）.
// [EN] Convenience POST (fixed-method DoJSON).
func (c *Client) PostJSON(ctx context.Context, url string, in, out any, headers map[string]string) error {
	return c.DoJSON(ctx, MethodPost, url, in, out, headers)
}
