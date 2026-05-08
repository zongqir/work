package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"work/notification/code/contract"
	"work/notification/code/dao"
	"work/notification/code/model"
	"work/notification/code/render"
)

type MessageConfig struct {
	RealtimeEnabled        bool                 `json:"realtime_enabled"`
	AggregateEnabled       bool                 `json:"aggregate_enabled"`
	Filter                 json.RawMessage      `json:"filter"`
	AggregatePeriodMinutes int                  `json:"aggregate_period_minutes"`
	Channel                render.ChannelPolicy `json:"channel"`
}

func (c *MessageConfig) EffectiveChannel() (render.ChannelPolicy, bool) {
	if c == nil {
		return render.ChannelPolicy{}, false
	}
	if c.Channel.Channel == "" {
		return render.ChannelPolicy{}, false
	}
	return c.Channel, true
}

func ParseMessageConfig(raw json.RawMessage) (*MessageConfig, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var cfg MessageConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("%w: parse message config: %v", contract.ErrUnsupportedConfig, err)
	}
	return &cfg, nil
}

func LoadMessageConfig(
	ctx context.Context,
	tenantID, messageType string,
	cache *Cache,
	loadAll func(context.Context) (map[string]map[string]json.RawMessage, error),
	logError func(context.Context, string, error),
) (*MessageConfig, error) {
	if loadAll == nil {
		return nil, fmt.Errorf("%w: load_all is required", contract.ErrInvalidRequest)
	}
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenant_id is required", contract.ErrInvalidRequest)
	}
	if messageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", contract.ErrInvalidRequest)
	}

	var raw json.RawMessage
	var err error
	if cache != nil {
		raw, err = cache.Pick(ctx, tenantID, messageType, loadAll, logError)
	} else {
		var all map[string]map[string]json.RawMessage
		all, err = loadAll(ctx)
		if err == nil {
			raw = all[tenantID][messageType]
		}
	}
	if err != nil {
		return nil, err
	}
	return ParseMessageConfig(raw)
}

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
		if err != nil && !errors.Is(err, dao.ErrNotFound) {
			return nil, err
		}
		if cfg, ok := findMessageConfig(items, messageType); ok {
			return runtimeMessageConfig(cfg), nil
		}
	}
	if cfg, ok := findMessageConfig(l.Defaults, messageType); ok {
		return runtimeMessageConfig(cfg), nil
	}

	return nil, fmt.Errorf("%w: message config not found: %s", contract.ErrUnsupportedConfig, messageType)
}

func LoadDefaultMessageConfigsFromFile(path string) ([]model.MessageConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: default_config_path is required", contract.ErrInvalidRequest)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var items []model.MessageConfig
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("%w: parse default message configs: %v", contract.ErrUnsupportedConfig, err)
	}
	return items, nil
}

func findMessageConfig(items []model.MessageConfig, messageType string) (*model.MessageConfig, bool) {
	for i := range items {
		if items[i].MessageType == messageType {
			return &items[i], true
		}
	}
	return nil, false
}

func runtimeMessageConfig(item *model.MessageConfig) *MessageConfig {
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
