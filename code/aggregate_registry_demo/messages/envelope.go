package messages

import "encoding/json"

// BizAggregateResult 是业务方聚合接口返回结果。
// 顶层固定为 message_type + payload。
type BizAggregateResult struct {
	MessageType string          `json:"message_type"`
	Payload     json.RawMessage `json:"payload"`
}

// BizAggregateResultMeta 描述某一种业务聚合结果该怎么解码。
type BizAggregateResultMeta struct {
	NewPayload     func() any
	BuildSMSParams func(windowLabel string, payload any) (map[string]string, error)
}
