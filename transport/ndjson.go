/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-07-28 21:03:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-07-28 21:03:00
 * @FilePath: \go-llmx\transport\ndjson.go
 * @Description: NDJSON 流式解析器 —— 每行一个 JSON 对象（Ollama 等本地推理协议）：
 * 逐行回调、CRLF 容忍、末行无换行符收口. 与 SSE 共享 Client 流式请求骨架
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package transport

import (
	"io"
	"strings"
)

// ReadNDJSON 解析 NDJSON 流并逐行回调.
// [EN] Parse an NDJSON stream, invoking the callback per line.
//
// 终止语义与 ReadSSE 一致：回调返回 io.EOF 视为正常结束（本函数返回 nil，
// 后续行不再消费）；返回其它 error 立即中断并原样透传；流读完自然返回 nil；
// 末行无尾随换行符时作为最后一行触发回调
func ReadNDJSON(r io.Reader, on func(line string) error) error {
	var (
		chunk []byte
		buf   [4096]byte
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
				line := strings.TrimRight(string(chunk[:idx]), "\r")
				chunk = chunk[idx+1:]
				if err := emitNDJSONLine(line, on); err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				// 末行无换行符：剩余内容作为最后一行收口
				// [EN] Trailing line without newline: emit as the final line.
				if err := emitNDJSONLine(strings.TrimRight(string(chunk), "\r"), on); err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
				return nil
			}
			return readErr
		}
	}
}

// emitNDJSONLine 单行分发（空行跳过；回调错误原样透传，io.EOF 由上层收口）.
// [EN] Emit one line (blank skipped; callback errors passed through).
func emitNDJSONLine(line string, on func(line string) error) error {
	if line == "" {
		return nil
	}
	return on(line)
}
