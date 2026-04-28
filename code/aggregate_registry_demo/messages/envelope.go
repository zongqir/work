package messages

type TemplateVars map[string]any

// BizAggregateResult 是业务方聚合接口返回结果。
// 业务返回 message_type + biz_vars，平台再补 system_vars。
type BizAggregateResult struct {
	MessageType string       `json:"message_type"`
	BizVars     TemplateVars `json:"biz_vars"`
}
