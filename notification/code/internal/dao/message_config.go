package dao

import (
	"context"

	"work/notification/code/internal/model"
)

type TenantMessageConfigStore interface {
	ListTenantMessageConfigs(ctx context.Context, tenantID string) ([]model.MessageConfig, error)
	SaveTenantMessageConfig(ctx context.Context, item *model.MessageConfig) error
	DeleteTenantMessageConfig(ctx context.Context, tenantID, messageType string) error
}
