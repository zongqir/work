package config

import (
	"context"
	"errors"
	"fmt"

	"work/notification/code/contract"
	"work/notification/code/dao"
	"work/notification/code/model"
	"work/notification/code/render"
)

type MessageConfigLoader struct {
	Defaults []model.MessageConfig
	Store    dao.TenantMessageConfigStore
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

	if l.Store != nil {
		items, err := l.Store.ListTenantMessageConfigs(ctx, tenantID, dao.MessageConfigQuery{MessageType: messageType})
		if err != nil && !errorsIsNotFound(err) {
			return nil, err
		}
		if cfg, ok := findMessageConfig(items, messageType); ok {
			return messageConfigFromTenantRecord(cfg), nil
		}
	}
	if cfg, ok := findMessageConfig(l.Defaults, messageType); ok {
		return messageConfigFromTenantRecord(cfg), nil
	}

	return nil, fmt.Errorf("%w: message config not found: %s", contract.ErrUnsupportedConfig, messageType)
}

func findMessageConfig(items []model.MessageConfig, messageType string) (*model.MessageConfig, bool) {
	for i := range items {
		if items[i].MessageType == messageType {
			return &items[i], true
		}
	}
	return nil, false
}

func messageConfigFromTenantRecord(item *model.MessageConfig) *MessageConfig {
	if item == nil {
		return nil
	}
	return &MessageConfig{
		RealtimeEnabled:        item.RealtimeEnabled,
		AggregateEnabled:       item.AggregateEnabled,
		AggregatePeriodMinutes: item.AggregatePeriodMinutes,
		Filter:                 item.Filter,
		Channel:                renderChannelPolicy(item.Channel),
	}
}

func renderChannelPolicy(channel model.ChannelPolicy) render.ChannelPolicy {
	return render.ChannelPolicy{
		Channel:      channel.Channel,
		TemplateCode: channel.TemplateCode,
		TemplateKey:  channel.TemplateKey,
		Audience: render.AudienceConfig{
			To:         channel.Audience.To,
			Cc:         channel.Audience.Cc,
			Bcc:        channel.Audience.Bcc,
			Recipients: channel.Audience.Recipients,
			Phone:      channel.Audience.Phone,
		},
		Delivery: render.DeliveryConfig{
			Platform: channel.Delivery.Platform,
			Secret:   channel.Delivery.Secret,
			AgentID:  channel.Delivery.AgentID,
			URL:      channel.Delivery.URL,
			Headers:  channel.Delivery.Headers,
		},
	}
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, dao.ErrNotFound)
}
