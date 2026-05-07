package contract

import "time"

type TemplateVars map[string]any

// BizAggregateRequest 是发给业务方聚合接口的请求。
type BizAggregateRequest struct {
	TenantID    string    `json:"tenant_id"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Filter      any       `json:"filter,omitempty"`
}

// BizAggregateResult 是业务方聚合接口返回结果。
// 业务只返回 biz_vars，message_type 由 handler 自身声明，平台再补 system_vars。
type BizAggregateResult struct {
	BizVars TemplateVars `json:"biz_vars"`
}

type RealtimeRequest struct {
	TenantID string
	Filter   any
	Event    any
}

type RealtimeResult struct {
	Matched        bool         `json:"matched"`
	IdempotencyKey string       `json:"idempotency_key,omitempty"`
	BizVars        TemplateVars `json:"biz_vars,omitempty"`
}
