package messages

type TemplateVars map[string]any

// BizAggregateResult 是业务方聚合接口返回结果。
// 业务只返回 biz_vars，message_type 由 handler 自身声明，平台再补 system_vars。
type BizAggregateResult struct {
	BizVars TemplateVars `json:"biz_vars"`
}
