package dao

import (
	"context"
	"errors"

	"work/notification/code/model"
)

var ErrNotFound = errors.New("dao: not found")

type MessageConfigQuery struct {
	MessageType string
}

type TenantMessageConfigStore interface {
	ListTenantMessageConfigs(ctx context.Context, tenantID string, query MessageConfigQuery) ([]model.MessageConfig, error)
	SaveTenantMessageConfig(ctx context.Context, item *model.MessageConfig) error
	DeleteTenantMessageConfig(ctx context.Context, tenantID, messageType string) error
}
