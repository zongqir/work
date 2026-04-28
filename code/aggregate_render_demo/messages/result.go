package main

// AggregateResult 是上游返回结果。
// 顶层固定有一个 message_type，再带一个具体消息体。
type AggregateResult struct {
	MessageType   string         `json:"message_type"`
	XdrRiskDigest *XdrRiskDigest `json:"xdr_risk_digest,omitempty"`
}
