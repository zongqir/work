package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"work/notification/code/contract"
	"work/notification/code/dao"
)

type DefaultMessageConfigLoader func(ctx context.Context, messageType string) (json.RawMessage, error)

type MessageConfigLoader struct {
	Default DefaultMessageConfigLoader
	Store   dao.TenantMessageConfigStore
}

func (l *MessageConfigLoader) Load(ctx context.Context, tenantID, messageType string) (*MessageConfig, error) {
	if l == nil {
		return nil, fmt.Errorf("%w: message_config_loader is required", contract.ErrInvalidRequest)
	}
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenant_id is required", contract.ErrInvalidRequest)
	}
	if messageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", contract.ErrInvalidRequest)
	}

	var tenantCfg *dao.TenantMessageConfig
	if l.Store != nil {
		var err error
		tenantCfg, err = l.Store.GetTenantMessageConfig(ctx, tenantID, messageType)
		if err != nil {
			if !errorsIsNotFound(err) {
				return nil, err
			}
			tenantCfg = nil
		}
	}
	if tenantCfg != nil {
		return messageConfigFromTenantRecord(tenantCfg), nil
	}
	if l.Default == nil {
		return nil, fmt.Errorf("%w: default config loader is required", contract.ErrInvalidRequest)
	}

	baseRaw, err := l.Default(ctx, messageType)
	if err != nil {
		return nil, err
	}

	return ParseMessageConfig(baseRaw)
}

func messageConfigFromTenantRecord(item *dao.TenantMessageConfig) *MessageConfig {
	if item == nil {
		return nil
	}
	return &MessageConfig{
		RealtimeEnabled:        item.RealtimeEnabled,
		AggregateEnabled:       item.AggregateEnabled,
		AggregatePeriodMinutes: item.AggregatePeriodMinutes,
		Filter:                 item.Filter,
		Channel:                item.Channel,
	}
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, dao.ErrNotFound)
}
