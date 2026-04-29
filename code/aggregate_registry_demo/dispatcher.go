package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// MessagePublisher 由平台提供，负责把命中的实时结果发布出去。
// 默认口径可以是发到 MQ，不承诺具体介质。
type MessagePublisher interface {
	Publish(ctx context.Context, msg *DispatchMessage) error
}

type Dispatcher struct {
	Publisher MessagePublisher
	LoadAll   func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	LogError  func(ctx context.Context, msg string, err error)

	cache configCache
}

type messageConfig struct {
	Enabled         bool            `json:"enabled"`
	AggregateFilter json.RawMessage `json:"aggregate_filter"`
	RealtimeFilter  json.RawMessage `json:"realtime_filter"`
}

func (d *Dispatcher) SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) error {
	handler, config, err := d.prepare(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if config == nil || !config.Enabled {
		return nil
	}

	result, err := handler.Aggregate(ctx, &BizAggregateRequest{
		TenantID:    tenantID,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		ConfigBody:  config.AggregateFilter,
	})
	if err != nil {
		return err
	}
	if result == nil || len(result.BizVars) == 0 {
		return nil
	}

	return d.Publisher.Publish(ctx, &DispatchMessage{
		TenantID:    tenantID,
		MessageType: handler.MessageType(),
		BizVars:     result.BizVars,
	})
}

func (d *Dispatcher) SendRealtime(ctx context.Context, tenantID, messageType string, eventBody json.RawMessage) error {
	handler, config, err := d.prepare(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if config == nil || !config.Enabled {
		return nil
	}

	decision, err := handler.Evaluate(ctx, &RealtimeRequest{
		TenantID:    tenantID,
		FilterQuery: config.RealtimeFilter,
		EventBody:   eventBody,
	})
	if err != nil {
		return err
	}
	if decision == nil {
		return fmt.Errorf("%w: realtime decision is nil", ErrTemporaryFailure)
	}
	if !decision.Matched {
		return nil
	}

	return d.Publisher.Publish(ctx, &DispatchMessage{
		TenantID:    tenantID,
		MessageType: handler.MessageType(),
		BizVars:     decision.BizVars,
		EventBody:   eventBody,
	})
}

func (d *Dispatcher) prepare(ctx context.Context, tenantID, messageType string) (Handler, *messageConfig, error) {
	if d == nil || d.Publisher == nil {
		return nil, nil, fmt.Errorf("%w: message publisher is required", ErrInvalidRequest)
	}
	if d.LoadAll == nil {
		return nil, nil, fmt.Errorf("%w: load_all is required", ErrInvalidRequest)
	}

	if tenantID == "" {
		return nil, nil, fmt.Errorf("%w: tenant_id is required", ErrInvalidRequest)
	}
	if messageType == "" {
		return nil, nil, fmt.Errorf("%w: message_type is required", ErrInvalidRequest)
	}

	handler, err := Resolve(messageType)
	if err != nil {
		return nil, nil, err
	}

	configBody, err := d.cache.pick(ctx, tenantID, messageType, d.LoadAll, d.LogError)
	if err != nil {
		return nil, nil, err
	}
	if len(configBody) == 0 {
		return handler, nil, nil
	}

	var config messageConfig
	if err := json.Unmarshal(configBody, &config); err != nil {
		return nil, nil, err
	}
	return handler, &config, nil
}
