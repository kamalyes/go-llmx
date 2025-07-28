/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-28 20:51:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-28 20:58:00
 * @FilePath: \go-llmx\transport\sse.go
 * @Description: SSE 流式解析器 —— 标准协议逐帧解析：
 * data:/event: 行、冒号注释行（网关心跳）、空行事件边界、多行 data 按 \n 拼接.
 * 全部流式适配器共享本实现，适配器只写协议 JSON 编解码
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package transport

import (
	"io"
	"strings"
)

// SSEEvent Server-Sent Events 协议的一帧事件.
// [EN] A single Server-Sent Events frame.
type SSEEvent struct {
	// Event event: 行的值（协议未指定时为空串）.
	// [EN] Value of the event: line, empty if unspecified.
	Event string

	// Data data: 行内容（多行 data 按 \n 拼接）.
	// [EN] Content of data: lines (joined by \n when multiple).
	Data string
}

// ReadSSE 解析 SSE 流并逐事件回调.
// [EN] Parse an SSE stream, invoking the callback per event.
//
// 终止语义：回调返回 io.EOF 视为正常结束（本函数返回 nil）；
// 返回其它 error 立即中断并原样透传；流读完自然返回 nil.
// OpenAI 的 [DONE] 标记由适配器在回调内识别并返回 io.EOF 实现
func ReadSSE(r io.Reader, on func(ev SSEEvent) error) error {
	var (
		event string
		data  []string
		chunk []byte
		buf   [2048]byte
	)

	for {
		n, readErr := r.Read(buf[:])
		if n > 0 {
			chunk = append(chunk, buf[:n]...)
			for {
				idx := indexByte(chunk, '\n')
				if idx < 0 {
					break
				}
				line := string(chunk[:idx])
				chunk = chunk[idx+1:]
				line = strings.TrimRight(line, "\r")

				switch {
				case line == "":
					// 空行：事件边界，完整事件触发回调.
					// [EN] Empty line: event boundary, fire the callback.
					if len(data) > 0 || event != "" {
						if err := on(SSEEvent{Event: event, Data: strings.Join(data, "\n")}); err != nil {
							if err == io.EOF {
								return nil
							}
							return err
						}
					}
					event, data = "", nil

				case strings.HasPrefix(line, ":"):
					// 注释行（网关心跳），忽略.
					// [EN] Comment line (gateway heartbeat), ignored.

				case strings.HasPrefix(line, SSEDataPrefix):
					data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, SSEDataPrefix), " "))

				case strings.HasPrefix(line, SSEEventPrefix):
					event = strings.TrimPrefix(strings.TrimPrefix(line, SSEEventPrefix), " ")
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}

// IsDoneMarker SSE 数据是否为 OpenAI 兼容流的结束标记 [DONE].
// [EN] Whether the SSE data is the OpenAI-compatible [DONE] marker.
func IsDoneMarker(data string) bool {
	return data == SSEDoneMarker
}

// indexByte 等价 bytes.IndexByte（内部零依赖实现）.
// [EN] Equivalent of bytes.IndexByte.
func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}
