package messages

// XdrRiskDigest 是 xdr_risk_digest 这类消息对应的返回结构。
type XdrRiskDigest struct {
	TotalCount    int                    `json:"total_count"`
	CategoryCount int                    `json:"category_count"`
	Examples      []XdrRiskDigestExample `json:"examples"`
}

// XdrRiskDigestExample 是案例明细。
type XdrRiskDigestExample struct {
	ObjectName string `json:"object_name"`
	RiskType   string `json:"risk_type"`
	EventCount int    `json:"event_count"`
}
