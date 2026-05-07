package config

import (
	"context"
	"encoding/json"
	"fmt"

	"work/notification/code/contract"
	"work/notification/code/render"
)

type MessageConfig struct {
	RealtimeEnabled        bool                   `json:"realtime_enabled"`
	AggregateEnabled       bool                   `json:"aggregate_enabled"`
	Filter                 json.RawMessage        `json:"filter"`
	AggregatePeriodMinutes int                    `json:"aggregate_period_minutes"`
	Channels               []render.ChannelPolicy `json:"channels"`
}

func ParseMessageConfig(raw json.RawMessage) (*MessageConfig, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var cfg MessageConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
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
