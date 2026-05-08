package dao

import (
	"context"
	"time"
)

type AggregateWatermark struct {
	TenantID      string
	MessageType   string
	LastWindowEnd time.Time
	UpdatedAt     time.Time
}

type AggregateWatermarkStore interface {
	LastWindowEnd(ctx context.Context, tenantID, messageType string) (time.Time, error)
	SaveWindowEnd(ctx context.Context, tenantID, messageType string, windowEnd time.Time) error
}
