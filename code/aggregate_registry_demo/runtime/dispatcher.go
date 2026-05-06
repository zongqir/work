package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"notes/code/aggregate_registry_demo/config"
	"notes/code/aggregate_registry_demo/contract"
)

// MessagePublisher 由平台提供，负责把命中的实时结果发布出去。
// 默认口径可以是发到 MQ，不承诺具体介质。
type MessagePublisher interface {
	Publish(ctx context.Context, msg *contract.DispatchMessage) error
}

type Dispatcher struct {
	Publisher            MessagePublisher
	LoadAll              func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	LogError             func(ctx context.Context, msg string, err error)
	RealtimeExpireAfter  time.Duration
	AggregateExpireAfter time.Duration
	Now                  func() time.Time

	cache config.Cache
}

type Options struct {
	Publisher            MessagePublisher
	LoadAll              func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	LogError             func(ctx context.Context, msg string, err error)
	CacheTTL             time.Duration
	CacheMaxStale        time.Duration
	RealtimeExpireAfter  time.Duration
	AggregateExpireAfter time.Duration
	Now                  func() time.Time
}

func NewDispatcher(options Options) *Dispatcher {
	d := &Dispatcher{
		Publisher:            options.Publisher,
		LoadAll:              options.LoadAll,
		LogError:             options.LogError,
		RealtimeExpireAfter:  options.RealtimeExpireAfter,
		AggregateExpireAfter: options.AggregateExpireAfter,
		Now:                  options.Now,
	}
	d.cache.TTL = options.CacheTTL
	d.cache.MaxStale = options.CacheMaxStale
	if options.Now != nil {
		d.cache.Now = options.Now
	}
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
	filter, err := parseHandlerPayload(handler, "filter", handler.NewFilter(), config.Filter)
	if err != nil {
		return err
	}

	result, err := handler.Aggregate(ctx, &contract.BizAggregateRequest{
		TenantID:    tenantID,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Filter:      filter,
	})
	if err != nil {
		return err
	}
	if result == nil || len(result.BizVars) == 0 {
		return nil
	}

	now := time.Now
	if d.Now != nil {
		now = d.Now
	}
	createdAt := now()
	expireAfter := d.AggregateExpireAfter
	if expireAfter <= 0 {
		expireAfter = 30 * time.Minute
	}

	return d.Publisher.Publish(ctx, &contract.DispatchMessage{
		IdempotencyKey: buildAggregateIdempotencyKey(tenantID, handler.MessageType(), windowStart, windowEnd),
		TenantID:       tenantID,
		MessageType:    handler.MessageType(),
		Source:         contract.DispatchSourceAggregate,
		RetryCount:     0,
		CreatedAt:      createdAt,
		ExpectedSendAt: createdAt,
		ExpireAt:       createdAt.Add(expireAfter),
		BizVars:        result.BizVars,
	})
}

func (d *Dispatcher) SendRealtime(ctx context.Context, tenantID, messageType string, event any) error {
	handler, config, err := d.prepare(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if config == nil || !config.Enabled {
		return nil
	}
	filter, err := parseHandlerPayload(handler, "filter", handler.NewFilter(), config.Filter)
	if err != nil {
		return err
	}
	realtimeReq := &contract.RealtimeRequest{
		TenantID: tenantID,
		Filter:   filter,
		Event:    event,
	}

	decision, err := handler.Evaluate(ctx, realtimeReq)
	if err != nil {
		return err
	}
	if decision == nil {
		return fmt.Errorf("%w: realtime decision is nil", contract.ErrTemporaryFailure)
	}
	if !decision.Matched {
		return nil
	}
	if decision.IdempotencyKey == "" {
		return fmt.Errorf("%w: idempotency_key is required", contract.ErrInvalidRequest)
	}

	now := time.Now
	if d.Now != nil {
		now = d.Now
	}
	createdAt := now()
	expireAfter := d.RealtimeExpireAfter
	if expireAfter <= 0 {
		expireAfter = 5 * time.Minute
	}

	return d.Publisher.Publish(ctx, &contract.DispatchMessage{
		IdempotencyKey: buildRealtimeIdempotencyKey(tenantID, handler.MessageType(), decision.IdempotencyKey),
		TenantID:       tenantID,
		MessageType:    handler.MessageType(),
		Source:         contract.DispatchSourceRealtime,
		RetryCount:     0,
		CreatedAt:      createdAt,
		ExpectedSendAt: createdAt,
		ExpireAt:       createdAt.Add(expireAfter),
		BizVars:        decision.BizVars,
		EventBody:      marshalEventBody(event),
	})
}

func (d *Dispatcher) prepare(ctx context.Context, tenantID, messageType string) (contract.Handler, *config.MessageConfig, error) {
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

	configBody, err := d.cache.Pick(ctx, tenantID, messageType, d.LoadAll, d.LogError)
	if err != nil {
		return nil, nil, err
	}
	if len(configBody) == 0 {
		return handler, nil, nil
	}

	var config config.MessageConfig
	if err := json.Unmarshal(configBody, &config); err != nil {
		return nil, nil, err
	}
	return handler, &config, nil
}

func parseHandlerPayload(handler contract.Handler, name string, target any, raw json.RawMessage) (any, error) {
	if target == nil {
		return nil, nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		return target, nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return nil, fmt.Errorf("%w: parse %s for %s: %v", contract.ErrInvalidRequest, name, handler.MessageType(), err)
	}
	return target, nil
}

func marshalEventBody(event any) json.RawMessage {
	if raw, ok := event.(json.RawMessage); ok {
		return raw
	}
	data, _ := json.Marshal(event)
	return data
}

func buildAggregateIdempotencyKey(tenantID, messageType string, windowStart, windowEnd time.Time) string {
	return fmt.Sprintf(
		"aggregate:%s:%s:%s:%s",
		tenantID,
		messageType,
		windowStart.UTC().Format(time.RFC3339Nano),
		windowEnd.UTC().Format(time.RFC3339Nano),
	)
}

func buildRealtimeIdempotencyKey(tenantID, messageType, bizKey string) string {
	return fmt.Sprintf("realtime:%s:%s:%s", tenantID, messageType, bizKey)
}
