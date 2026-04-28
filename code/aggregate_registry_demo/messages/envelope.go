package main

import "encoding/json"

// AggregateEnvelope 是上游返回结果。
// 顶层固定为 message_type + payload。
type AggregateEnvelope struct {
	MessageType string          `json:"message_type"`
	Payload     json.RawMessage `json:"payload"`
}

// MessageSpec 描述某一种消息该怎么解码、该用哪套模板。
type MessageSpec struct {
	TemplateCode string
	NewPayload   func() any
}
