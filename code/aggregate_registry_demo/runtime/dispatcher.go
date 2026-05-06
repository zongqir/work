package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	Publisher            MessagePublisher
	LoadAll              func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	LogError             func(ctx context.Context, msg string, err error)
	RealtimeExpireAfter  time.Duration
	AggregateExpireAfter time.Duration
	Now                  func() time.Time

	cache configCache
}

type messageConfig struct {
	Enabled                bool            `json:"enabled"`
	Filter                 json.RawMessage `json:"filter"`
	AggregateFilter        json.RawMessage `json:"aggregate_filter"`
	AggregatePeriodMinutes int             `json:"aggregate_period_minutes"`
	RealtimeFilter         json.RawMessage `json:"realtime_filter"`
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
		d.cache.now = options.Now
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
	filter, err := parseHandlerPayload(handler, "filter", handler.NewFilter(), config.filterForAggregate())
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
	messageID, err := newMessageID()
	if err != nil {
		return err
	}

	return d.Publisher.Publish(ctx, &contract.DispatchMessage{
		MessageID:      messageID,
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

func (d *Dispatcher) SendRealtime(ctx context.Context, tenantID, messageType string, eventBody json.RawMessage) error {
	handler, config, err := d.prepare(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if config == nil || !config.Enabled {
		return nil
	}
	filter, err := parseHandlerPayload(handler, "filter", handler.NewFilter(), config.filterForRealtime())
	if err != nil {
		return err
	}
	event, err := parseHandlerPayload(handler, "realtime event", handler.NewRealtimeEvent(), eventBody)
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

	bizKey, err := handler.RealtimeIdempotencyKey(ctx, realtimeReq)
	if err != nil {
		return err
	}
	if bizKey == "" {
		return fmt.Errorf("%w: biz_idempotency_key is required", contract.ErrInvalidRequest)
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
	messageID, err := newMessageID()
	if err != nil {
		return err
	}

	return d.Publisher.Publish(ctx, &contract.DispatchMessage{
		MessageID:      messageID,
		IdempotencyKey: buildRealtimeIdempotencyKey(tenantID, handler.MessageType(), bizKey),
		TenantID:       tenantID,
		MessageType:    handler.MessageType(),
		Source:         contract.DispatchSourceRealtime,
		RetryCount:     0,
		CreatedAt:      createdAt,
		ExpectedSendAt: createdAt,
		ExpireAt:       createdAt.Add(expireAfter),
		BizVars:        decision.BizVars,
		EventBody:      eventBody,
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

func newMessageID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
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

func (c messageConfig) filterForAggregate() json.RawMessage {
	if len(c.Filter) != 0 {
		return c.Filter
	}
	return c.AggregateFilter
}

func (c messageConfig) filterForRealtime() json.RawMessage {
	if len(c.Filter) != 0 {
		return c.Filter
	}
	return c.RealtimeFilter
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
