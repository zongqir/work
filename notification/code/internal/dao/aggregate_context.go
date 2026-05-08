package dao

import (
	"context"
	"time"
)

type AggregateContext struct {
	IdempotencyKey string
	TenantID       string
	MessageType    string
	WindowStart    time.Time
	WindowEnd      time.Time
	CreatedAt      time.Time
	ExpireAt       time.Time
}

type AggregateContextStore interface {
	SaveAggregateContext(ctx context.Context, item *AggregateContext) error
	GetAggregateContext(ctx context.Context, idempotencyKey string) (*AggregateContext, error)
}
