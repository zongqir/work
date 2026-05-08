package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"work/notification/code/config"
	"work/notification/code/pkg/notification/contract"
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

	cache config.Cache
}

// SendAggregate returns true once the aggregate window has been handled,
// even if the handler produced no biz vars and nothing was published.
func (d *Dispatcher) SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) (bool, error) {
	spec, aggregate, err := contract.ResolveAggregate(messageType)
	if err != nil {
		return false, err
	}
	cfg, err := d.loadConfig(ctx, tenantID, messageType)
	if err != nil {
		return false, err
	}
	if cfg == nil || !cfg.AggregateEnabled {
		return false, nil
	}
	filter, err := parseHandlerPayload(spec, spec.NewFilter(), cfg.Filter)
	if err != nil {
		return false, err
	}

	aggregateResult, err := aggregate.Aggregate(ctx, &contract.BizAggregateRequest{
		TenantID:    tenantID,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Filter:      filter,
	})
	if err != nil {
		return false, err
	}
	if aggregateResult == nil || len(aggregateResult.BizVars) == 0 {
		return true, nil
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

	err = d.Publisher.Publish(ctx, &contract.DispatchMessage{
		IdempotencyKey: buildAggregateIdempotencyKey(tenantID, spec.MessageType(), windowStart, windowEnd),
		TenantID:       tenantID,
		MessageType:    spec.MessageType(),
		Source:         contract.DispatchSourceAggregate,
		RetryCount:     0,
		CreatedAt:      createdAt,
		ExpectedSendAt: createdAt,
		ExpireAt:       createdAt.Add(expireAfter),
		BizVars:        aggregateResult.BizVars,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *Dispatcher) SendRealtime(ctx context.Context, tenantID, messageType string, event any) error {
	spec, realtime, err := contract.ResolveRealtime(messageType)
	if err != nil {
		return err
	}
	cfg, err := d.loadConfig(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if cfg == nil || !cfg.RealtimeEnabled {
		return nil
	}
	filter, err := parseHandlerPayload(spec, spec.NewFilter(), cfg.Filter)
	if err != nil {
		return err
	}
	realtimeReq := &contract.RealtimeRequest{
		TenantID: tenantID,
		Filter:   filter,
		Event:    event,
	}

	realtimeResult, err := realtime.Evaluate(ctx, realtimeReq)
	if err != nil {
		return err
	}
	if realtimeResult == nil {
		return fmt.Errorf("%w: realtime result is nil", contract.ErrTemporaryFailure)
	}
	if !realtimeResult.Matched {
		return nil
	}
	if realtimeResult.IdempotencyKey == "" {
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

	err = d.Publisher.Publish(ctx, &contract.DispatchMessage{
		IdempotencyKey: buildRealtimeIdempotencyKey(tenantID, spec.MessageType(), realtimeResult.IdempotencyKey),
		TenantID:       tenantID,
		MessageType:    spec.MessageType(),
		Source:         contract.DispatchSourceRealtime,
		RetryCount:     0,
		CreatedAt:      createdAt,
		ExpectedSendAt: createdAt,
		ExpireAt:       createdAt.Add(expireAfter),
		BizVars:        realtimeResult.BizVars,
	})
	return err
}

func (d *Dispatcher) loadConfig(ctx context.Context, tenantID, messageType string) (*config.MessageConfig, error) {
	if d == nil || d.Publisher == nil {
		return nil, fmt.Errorf("%w: message publisher is required", contract.ErrInvalidRequest)
	}
	if d.LoadAll == nil {
		return nil, fmt.Errorf("%w: load_all is required", contract.ErrInvalidRequest)
	}

	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenant_id is required", contract.ErrInvalidRequest)
	}
	if messageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", contract.ErrInvalidRequest)
	}

	d.cache.TTL = d.CacheTTL
	d.cache.MaxStale = d.CacheMaxStale
	if d.Now != nil {
		d.cache.Now = d.Now
	}
	return config.LoadMessageConfig(ctx, tenantID, messageType, &d.cache, d.LoadAll, d.LogError)
}

func parseHandlerPayload(spec contract.MessageTypeSpec, target any, raw json.RawMessage) (any, error) {
	if target == nil {
		return nil, nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		return target, nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return nil, fmt.Errorf("%w: parse filter for %s: %v", contract.ErrInvalidRequest, spec.MessageType(), err)
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
