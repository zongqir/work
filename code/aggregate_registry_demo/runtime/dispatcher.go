package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"notes/code/aggregate_registry_demo/contract"
)

// MessagePublisher 由平台提供，负责把命中的实时结果发布出去。
// 默认口径可以是发到 MQ，不承诺具体介质。
type MessagePublisher interface {
	Publish(ctx context.Context, msg *contract.DispatchMessage) error
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

type Options struct {
	Publisher     MessagePublisher
	LoadAll       func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	LogError      func(ctx context.Context, msg string, err error)
	CacheTTL      time.Duration
	CacheMaxStale time.Duration
}

func NewDispatcher(options Options) *Dispatcher {
	d := &Dispatcher{
		Publisher: options.Publisher,
		LoadAll:   options.LoadAll,
		LogError:  options.LogError,
	}
	d.cache.TTL = options.CacheTTL
	d.cache.MaxStale = options.CacheMaxStale
	return d
}

func (d *Dispatcher) SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) error {
	handler, config, err := d.prepare(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if config == nil || !config.Enabled {
		return nil
	}

	result, err := handler.Aggregate(ctx, &contract.BizAggregateRequest{
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

	return d.Publisher.Publish(ctx, &contract.DispatchMessage{
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

	decision, err := handler.Evaluate(ctx, &contract.RealtimeRequest{
		TenantID:    tenantID,
		FilterQuery: config.RealtimeFilter,
		EventBody:   eventBody,
	})
	if err != nil {
		return err
	}
	if decision == nil {
		return fmt.Errorf("%w: realtime decision is nil", contract.ErrTemporaryFailure)
	}
	if !decision.Matched {
		return nil
	}

	return d.Publisher.Publish(ctx, &contract.DispatchMessage{
		TenantID:    tenantID,
		MessageType: handler.MessageType(),
		BizVars:     decision.BizVars,
		EventBody:   eventBody,
	})
}

func (d *Dispatcher) prepare(ctx context.Context, tenantID, messageType string) (contract.Handler, *messageConfig, error) {
	if d == nil || d.Publisher == nil {
		return nil, nil, fmt.Errorf("%w: message publisher is required", contract.ErrInvalidRequest)
	}
	if d.LoadAll == nil {
		return nil, nil, fmt.Errorf("%w: load_all is required", contract.ErrInvalidRequest)
	}

	if tenantID == "" {
		return nil, nil, fmt.Errorf("%w: tenant_id is required", contract.ErrInvalidRequest)
	}
	if messageType == "" {
		return nil, nil, fmt.Errorf("%w: message_type is required", contract.ErrInvalidRequest)
	}

	handler, err := contract.Resolve(messageType)
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
