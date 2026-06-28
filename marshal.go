/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-28 16:36:39
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-28 16:51:38
 * @FilePath: \go-llmx\marshal.go
 * @Description: 消息序列化 —— Message 与 JSON 往返（History 持久化的前置）.
 * Part 为值类型接口，wire 形态以 type 标记区分，四种 Part 无损往返
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package llmx

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// wirePart 序列化载体形态标记.
// [EN] Wire-part type markers.
const (
	partTypeText       = "text"
	partTypeImage      = "image"
	partTypeToolCall   = "tool_call"
	partTypeToolResult = "tool_result"
)

// wirePart 序列化载体（type 标记区分四种 Part）.
// [EN] Serialization carrier (four Part kinds marked by type).
type wirePart struct {
	Type string `json:"type"`

	// TextPart
	Text string `json:"text,omitempty"`

	// ImagePart
	ImageURL  string `json:"image_url,omitempty"`
	MIMEType  string `json:"mime_type,omitempty"`
	ImageData string `json:"image_data,omitempty"`

	// ToolCallPart
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Arguments  string `json:"arguments,omitempty"`

	// ToolResultPart
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// wireMessage 消息序列化载体.
// [EN] Message serialization carrier.
type wireMessage struct {
	Role    string     `json:"role"`
	Content []wirePart `json:"content"`
}

// MarshalMessage 消息序列化（JSON；可持久化到 History 后端）.
// [EN] Serialize a message (JSON; persistable into a History backend).
func MarshalMessage(msg Message) ([]byte, error) {
	w := wireMessage{Role: string(msg.Role)}
	for _, p := range msg.Content {
		switch v := p.(type) {
		case TextPart:
			w.Content = append(w.Content, wirePart{Type: partTypeText, Text: v.Text})
		case ImagePart:
			w.Content = append(w.Content, wirePart{
				Type:      partTypeImage,
				ImageURL:  v.URL,
				MIMEType:  v.MIMEType,
				ImageData: base64.StdEncoding.EncodeToString(v.Data),
			})
		case ToolCallPart:
			w.Content = append(w.Content, wirePart{
				Type:       partTypeToolCall,
				ToolCallID: v.ID,
				Name:       v.Name,
				Arguments:  v.Arguments,
			})
		case ToolResultPart:
			w.Content = append(w.Content, wirePart{
				Type:   partTypeToolResult,
				Result: v.Result,
				Error:  v.Error,
			})
		default:
			return nil, fmt.Errorf("%w: unsupported part type %T", ErrInvalidRequest, p)
		}
	}
	return json.Marshal(w)
}

// UnmarshalMessage 消息反序列化（type 标记还原 Part 具体类型）.
// [EN] Deserialize a message (restores concrete Part types).
func UnmarshalMessage(data []byte) (Message, error) {
	var w wireMessage
	if err := json.Unmarshal(data, &w); err != nil {
		return Message{}, err
	}
	msg := Message{Role: Role(w.Role)}
	for _, wp := range w.Content {
		switch wp.Type {
		case partTypeText:
			msg.Content = append(msg.Content, TextPart{Text: wp.Text})
		case partTypeImage:
			data, err := base64.StdEncoding.DecodeString(wp.ImageData)
			if err != nil {
				return Message{}, fmt.Errorf("%w: invalid base64 image data: %v", ErrInvalidRequest, err)
			}
			if len(data) == 0 {
				data = nil // 与零值 ImagePart.Data 对齐（空串解码为 nil）
			}
			msg.Content = append(msg.Content, ImagePart{
				URL:      wp.ImageURL,
				MIMEType: wp.MIMEType,
				Data:     data,
			})
		case partTypeToolCall:
			msg.Content = append(msg.Content, ToolCallPart{
				ID:        wp.ToolCallID,
				Name:      wp.Name,
				Arguments: wp.Arguments,
			})
		case partTypeToolResult:
			msg.Content = append(msg.Content, ToolResultPart{
				Result: wp.Result,
				Error:  wp.Error,
			})
		default:
			return Message{}, fmt.Errorf("%w: unknown part type %q", ErrInvalidRequest, wp.Type)
		}
	}
	return msg, nil
}
