package dao

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"work/notification/code/render"
)

var ErrNotFound = errors.New("dao: not found")

type TenantMessageConfig struct {
	TenantID               string
	MessageType            string
	RealtimeEnabled        bool
	AggregateEnabled       bool
	AggregatePeriodMinutes int
	Filter                 json.RawMessage
	Channel                render.ChannelPolicy
	UpdatedBy              string
	UpdatedAt              time.Time
}

type MessageConfigQuery struct {
	MessageType string
}

type TenantMessageConfigStore interface {
	ListTenantMessageConfigs(ctx context.Context, tenantID string, query MessageConfigQuery) ([]TenantMessageConfig, error)
	GetTenantMessageConfig(ctx context.Context, tenantID, messageType string) (*TenantMessageConfig, error)
	SaveTenantMessageConfig(ctx context.Context, item *TenantMessageConfig) error
	DeleteTenantMessageConfig(ctx context.Context, tenantID, messageType string) error
}
