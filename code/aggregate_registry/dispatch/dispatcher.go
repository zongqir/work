package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"notes/code/aggregate_registry/config"
	"notes/code/aggregate_registry/contract"
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
	CacheTTL             time.Duration
	CacheMaxStale        time.Duration
	RealtimeExpireAfter  time.Duration
	AggregateExpireAfter time.Duration
	Now                  func() time.Time

	cacheOnce sync.Once
	cache     config.Cache
}

func (d *Dispatcher) SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) error {
	handler, cfg, err := d.prepare(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if cfg == nil || !cfg.AggregateEnabled {
		return nil
	}
	filter, err := parseHandlerPayload(handler, handler.NewFilter(), cfg.Filter)
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
	handler, cfg, err := d.prepare(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if cfg == nil || !cfg.RealtimeEnabled {
		return nil
	}
	filter, err := parseHandlerPayload(handler, handler.NewFilter(), cfg.Filter)
	if err != nil {
		return err
	}
	realtimeReq := &contract.RealtimeRequest{
		TenantID: tenantID,
		Filter:   filter,
		Event:    event,
	}

	result, err := handler.Evaluate(ctx, realtimeReq)
	if err != nil {
		return err
	}
	if result == nil {
		return fmt.Errorf("%w: realtime result is nil", contract.ErrTemporaryFailure)
	}
	if !result.Matched {
		return nil
	}
	if result.IdempotencyKey == "" {
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
		IdempotencyKey: buildRealtimeIdempotencyKey(tenantID, handler.MessageType(), result.IdempotencyKey),
		TenantID:       tenantID,
		MessageType:    handler.MessageType(),
		Source:         contract.DispatchSourceRealtime,
		RetryCount:     0,
		CreatedAt:      createdAt,
		ExpectedSendAt: createdAt,
		ExpireAt:       createdAt.Add(expireAfter),
		BizVars:        result.BizVars,
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

	d.ensureCache()
	configBody, err := d.cache.Pick(ctx, tenantID, messageType, d.LoadAll, d.LogError)
	if err != nil {
		return nil, nil, err
	}
	if len(configBody) == 0 {
		return handler, nil, nil
	}

	var cfg config.MessageConfig
	if err := json.Unmarshal(configBody, &cfg); err != nil {
		return nil, nil, err
	}
	return handler, &cfg, nil
}

func (d *Dispatcher) ensureCache() {
	d.cacheOnce.Do(func() {
		d.cache.TTL = d.CacheTTL
		d.cache.MaxStale = d.CacheMaxStale
		if d.Now != nil {
			d.cache.Now = d.Now
		}
	})
}

func parseHandlerPayload(handler contract.Handler, target any, raw json.RawMessage) (any, error) {
	if target == nil {
		return nil, nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		return target, nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return nil, fmt.Errorf("%w: parse filter for %s: %v", contract.ErrInvalidRequest, handler.MessageType(), err)
	}
	return target, nil
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

