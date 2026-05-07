package contract

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidRequest           = errors.New("invalid request")
	ErrUnsupportedConfig        = errors.New("unsupported config")
	ErrTemporaryFailure         = errors.New("temporary failure")
	ErrHandlerNotFound          = errors.New("handler not found")
	ErrCapabilityNotImplemented = errors.New("capability not implemented")
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
	WindowStart    time.Time    `json:"window_start,omitempty"`
	WindowEnd      time.Time    `json:"window_end,omitempty"`
	BizVars        TemplateVars `json:"biz_vars"`
}

const (
	DispatchSourceAggregate = "aggregate"
	DispatchSourceRealtime  = "realtime"
)

type MessageTypeSpec interface {
	MessageType() string
	NewFilter() any
}

type RealtimeEvaluator interface {
	Evaluate(ctx context.Context, req *RealtimeRequest) (*RealtimeResult, error)
}

type AggregateProvider interface {
	Aggregate(ctx context.Context, req *BizAggregateRequest) (*BizAggregateResult, error)
}

// Handler 是同时支持实时和聚合的兼容契约。
type Handler interface {
	MessageTypeSpec
	RealtimeEvaluator
	AggregateProvider
}

type Registration struct {
	Spec      MessageTypeSpec
	Realtime  RealtimeEvaluator
	Aggregate AggregateProvider
}
