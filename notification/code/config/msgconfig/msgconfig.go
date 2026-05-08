package msgconfig

import (
	"context"
	"fmt"

	"work/notification/code/config"
	"work/notification/code/internal/model"
	"work/notification/code/pkg/notification/capability"
	"work/notification/code/pkg/notification/contract"
)

type Loader struct {
	ConfigLoader *config.MessageConfigLoader
}

func (l *Loader) LoadView(ctx context.Context, tenantID, messageType string) (*capability.MessageConfigView, error) {
	if l == nil || l.ConfigLoader == nil {
		return nil, fmt.Errorf("%w: config_loader is required", contract.ErrInvalidRequest)
	}

	cfg, err := l.ConfigLoader.LoadRecord(ctx, tenantID, messageType)
	if err != nil {
		return nil, err
	}
	capItem, err := capability.Get(messageType)
	if err != nil {
		return nil, err
	}
	return &capability.MessageConfigView{
		Capability: *capItem,
		Config:     toPublicConfig(cfg),
	}, nil
}

func Validate(item *model.MessageConfig) error {
	if item == nil {
		return fmt.Errorf("%w: message config is required", contract.ErrInvalidRequest)
	}
	return capability.ValidateConfig(&capability.MessageConfig{
		TenantID:               item.TenantID,
		MessageType:            item.MessageType,
		RealtimeEnabled:        item.RealtimeEnabled,
		AggregateEnabled:       item.AggregateEnabled,
		AggregatePeriodMinutes: item.AggregatePeriodMinutes,
		Filter:                 item.Filter,
		Channel:                toPublicChannel(item.Channel),
		UpdatedBy:              item.UpdatedBy,
		UpdatedAt:              item.UpdatedAt,
	})
}

func toPublicConfig(item *model.MessageConfig) capability.MessageConfig {
	if item == nil {
		return capability.MessageConfig{}
	}
	return capability.MessageConfig{
		TenantID:               item.TenantID,
		MessageType:            item.MessageType,
		RealtimeEnabled:        item.RealtimeEnabled,
		AggregateEnabled:       item.AggregateEnabled,
		AggregatePeriodMinutes: item.AggregatePeriodMinutes,
		Filter:                 item.Filter,
		Channel:                toPublicChannel(item.Channel),
		UpdatedBy:              item.UpdatedBy,
		UpdatedAt:              item.UpdatedAt,
	}
}

func toPublicChannel(item model.ChannelPolicy) capability.ChannelPolicy {
	return capability.ChannelPolicy{
		Channel:      item.Channel,
		TemplateCode: item.TemplateCode,
		TemplateKey:  item.TemplateKey,
		Audience:     toPublicAudience(item.Audience),
		Delivery:     toPublicDelivery(item.Delivery),
	}
}

func toPublicAudience(item model.AudienceConfig) capability.AudienceConfig {
	return capability.AudienceConfig{
		To:         item.To,
		Cc:         item.Cc,
		Bcc:        item.Bcc,
		Recipients: item.Recipients,
		Phone:      item.Phone,
	}
}

func toPublicDelivery(item model.DeliveryConfig) capability.DeliveryConfig {
	return capability.DeliveryConfig{
		Platform: item.Platform,
		Secret:   item.Secret,
		AgentID:  item.AgentID,
		URL:      item.URL,
		Headers:  item.Headers,
	}
}
