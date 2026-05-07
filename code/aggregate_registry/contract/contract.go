package contract

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidRequest    = errors.New("invalid request")
	ErrUnsupportedConfig = errors.New("unsupported config")
	ErrTemporaryFailure  = errors.New("temporary failure")
	ErrHandlerNotFound   = errors.New("handler not found")
)

type DispatchMessage struct {
	IdempotencyKey string       `json:"idempotency_key"`
	TenantID       string       `json:"tenant_id"`
	MessageType    string       `json:"message_type"`
	Source         string       `json:"source"`
	RetryCount     int          `json:"retry_count"`
	CreatedAt      time.Time    `json:"created_at"`
	ExpectedSendAt time.Time    `json:"expected_send_at"`
	ExpireAt       time.Time    `json:"expire_at"`
	BizVars        TemplateVars `json:"biz_vars"`
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
