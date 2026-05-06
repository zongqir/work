package contract

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrInvalidRequest     = errors.New("invalid aggregate request")
	ErrUnsupportedConfig  = errors.New("unsupported aggregate config")
	ErrTemporaryFailure   = errors.New("temporary aggregate failure")
	ErrAggregatorNotFound = errors.New("aggregator not found")
)

type DispatchMessage struct {
	MessageID      string          `json:"message_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	TenantID       string          `json:"tenant_id"`
	MessageType    string          `json:"message_type"`
	Source         string          `json:"source"`
	RetryCount     int             `json:"retry_count"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpectedSendAt time.Time       `json:"expected_send_at"`
	ExpireAt       time.Time       `json:"expire_at"`
	BizVars        TemplateVars    `json:"biz_vars"`
	EventBody      json.RawMessage `json:"event_body,omitempty"`
}

const (
	DispatchSourceAggregate = "aggregate"
	DispatchSourceRealtime  = "realtime"
)

// Handler 是业务侧需要实现的最小生产契约。
type Handler interface {
	MessageType() string
	NewFilter() any
	Aggregate(ctx context.Context, req *BizAggregateRequest) (*BizAggregateResult, error)
	Evaluate(ctx context.Context, req *RealtimeRequest) (*RealtimeResult, error)
}
