package contract

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"notes/code/aggregate_registry_demo/messages"
)

var (
	ErrInvalidRequest     = errors.New("invalid aggregate request")
	ErrUnsupportedConfig  = errors.New("unsupported aggregate config")
	ErrTemporaryFailure   = errors.New("temporary aggregate failure")
	ErrAggregatorNotFound = errors.New("aggregator not found")
)

// BizAggregateRequest 是发给业务方聚合接口的请求。
type BizAggregateRequest struct {
	TenantID    string    `json:"tenant_id"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Filter      any       `json:"filter,omitempty"`
}

type RealtimeRequest struct {
	TenantID string          `json:"tenant_id"`
	Filter   any             `json:"filter,omitempty"`
	Event    json.RawMessage `json:"event,omitempty"`
}

type RealtimeDecision struct {
	Matched        bool                  `json:"matched"`
	IdempotencyKey string                `json:"idempotency_key,omitempty"`
	BizVars        messages.TemplateVars `json:"biz_vars,omitempty"`
}

type DispatchMessage struct {
	MessageID      string                `json:"message_id"`
	IdempotencyKey string                `json:"idempotency_key"`
	TenantID       string                `json:"tenant_id"`
	MessageType    string                `json:"message_type"`
	Source         string                `json:"source"`
	RetryCount     int                   `json:"retry_count"`
	CreatedAt      time.Time             `json:"created_at"`
	ExpectedSendAt time.Time             `json:"expected_send_at"`
	ExpireAt       time.Time             `json:"expire_at"`
	BizVars        messages.TemplateVars `json:"biz_vars"`
	EventBody      json.RawMessage       `json:"event_body,omitempty"`
}

const (
	DispatchSourceAggregate = "aggregate"
	DispatchSourceRealtime  = "realtime"
)

// Handler 是业务侧需要实现的最小生产契约。
type Handler interface {
	MessageType() string
	NewFilter() any
	Aggregate(ctx context.Context, req *BizAggregateRequest) (*messages.BizAggregateResult, error)
	Evaluate(ctx context.Context, req *RealtimeRequest) (*RealtimeDecision, error)
}
