package messages

type TemplateVars map[string]any

// BizAggregateResult 是业务方聚合接口返回结果。
// 业务直接返回 message_type + template_vars，平台不再解码业务 payload。
type BizAggregateResult struct {
	MessageType  string       `json:"message_type"`
	TemplateVars TemplateVars `json:"template_vars"`
}
