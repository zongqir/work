package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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

	cache configCache
}

type sendMode string

const (
	sendModeAggregate sendMode = "aggregate"
	sendModeRealtime  sendMode = "realtime"
)

type messageConfig struct {
	Enabled         bool            `json:"enabled"`
	AggregateFilter json.RawMessage `json:"aggregate_filter"`
	RealtimeFilter  json.RawMessage `json:"realtime_filter"`
}

func (d *Dispatcher) SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) error {
	return d.send(ctx, sendModeAggregate, tenantID, messageType, nil, windowStart, windowEnd)
}

func (d *Dispatcher) SendRealtime(ctx context.Context, tenantID, messageType string, eventBody json.RawMessage) error {
	return d.send(ctx, sendModeRealtime, tenantID, messageType, eventBody, time.Time{}, time.Time{})
}

func (d *Dispatcher) send(ctx context.Context, mode sendMode, tenantID, messageType string, eventBody json.RawMessage, windowStart, windowEnd time.Time) error {
	if d == nil || d.Publisher == nil {
		return fmt.Errorf("%w: message publisher is required", ErrInvalidRequest)
	}
	if d.LoadAll == nil {
		return fmt.Errorf("%w: load_all is required", ErrInvalidRequest)
	}

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidRequest)
	}
	messageType = strings.TrimSpace(messageType)
	if messageType == "" {
		return fmt.Errorf("%w: message_type is required", ErrInvalidRequest)
	}

	handler, err := Resolve(messageType)
	if err != nil {
		return err
	}

	configBody, err := d.cache.pick(ctx, tenantID, messageType, d.LoadAll)
	if err != nil {
		return err
	}
	if len(configBody) == 0 {
		return nil
	}

	var config messageConfig
	if err := json.Unmarshal(configBody, &config); err != nil {
		return err
	}
	if !config.Enabled {
		return nil
	}

	switch mode {
	case sendModeAggregate:
		return d.sendAggregateWithHandler(ctx, handler, &BizAggregateRequest{
			TenantID:    tenantID,
			WindowStart: windowStart,
			WindowEnd:   windowEnd,
			ConfigBody:  config.AggregateFilter,
		})
	case sendModeRealtime:
		decision, err := d.sendRealtimeWithHandler(ctx, handler, &RealtimeRequest{
			TenantID:    tenantID,
			FilterQuery: config.RealtimeFilter,
			EventBody:   eventBody,
		})
		if err != nil {
			return err
		}
		if decision == nil {
			return nil
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported send mode", ErrInvalidRequest)
	}
}

func (d *Dispatcher) dispatch(ctx context.Context, msg *DispatchMessage) error {
	if d == nil || d.Publisher == nil {
		return fmt.Errorf("%w: message publisher is required", ErrInvalidRequest)
	}
	if msg == nil {
		return fmt.Errorf("%w: dispatch message is nil", ErrInvalidRequest)
	}

	return d.Publisher.Publish(ctx, msg)
}

func (d *Dispatcher) sendAggregateWithHandler(ctx context.Context, handler Handler, req *BizAggregateRequest) error {
	if handler == nil {
		return fmt.Errorf("%w: handler is required", ErrInvalidRequest)
	}
	if req == nil {
		return fmt.Errorf("%w: aggregate request is nil", ErrInvalidRequest)
	}

	result, err := handler.Aggregate(ctx, req)
	if err != nil {
		return err
	}
	if result == nil || len(result.BizVars) == 0 {
		return nil
	}

	messageType := result.MessageType
	if strings.TrimSpace(messageType) == "" {
		messageType = handler.MessageType()
	}

	return d.dispatch(ctx, &DispatchMessage{
		TenantID:    req.TenantID,
		MessageType: messageType,
		BizVars:     result.BizVars,
	})
}

func (d *Dispatcher) sendRealtimeWithHandler(ctx context.Context, handler Handler, req *RealtimeRequest) (*RealtimeDecision, error) {
	if handler == nil {
		return nil, fmt.Errorf("%w: handler is required", ErrInvalidRequest)
	}
	if req == nil {
		return nil, fmt.Errorf("%w: realtime request is nil", ErrInvalidRequest)
	}

	decision, err := handler.Evaluate(ctx, req)
	if err != nil {
		return nil, err
	}
	if decision == nil {
		return nil, fmt.Errorf("%w: realtime decision is nil", ErrTemporaryFailure)
	}
	if !decision.Matched {
		return decision, nil
	}

	if err := d.dispatch(ctx, &DispatchMessage{
		TenantID:    req.TenantID,
		MessageType: handler.MessageType(),
		BizVars:     decision.BizVars,
		EventBody:   req.EventBody,
	}); err != nil {
		return nil, err
	}

	return decision, nil
}

type configCache struct {
	TTL      time.Duration
	MaxStale time.Duration

	now func() time.Time
	mu  sync.RWMutex

	items      map[string]map[string]json.RawMessage
	loadedAt   time.Time
	refreshing bool
}

func (c *configCache) pick(ctx context.Context, tenantID, messageType string, loadAll func(context.Context) (map[string]map[string]json.RawMessage, error)) (json.RawMessage, error) {
	if c == nil {
		return nil, nil
	}
	if tenantID == "" || messageType == "" {
		return nil, nil
	}
	if loadAll == nil {
		return nil, fmt.Errorf("%w: load_all is required", ErrInvalidRequest)
	}

	nowFn := c.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()

	c.mu.RLock()
	items := c.items
	loadedAt := c.loadedAt
	refreshing := c.refreshing
	c.mu.RUnlock()

	ttl := c.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	maxStale := c.MaxStale
	if maxStale <= 0 {
		maxStale = 30 * time.Minute
	}

	age := now.Sub(loadedAt)
	if items == nil || loadedAt.IsZero() || age >= maxStale {
		all, err := loadAll(ctx)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.items = all
		c.loadedAt = nowFn()
		c.refreshing = false
		items = c.items
		c.mu.Unlock()
	} else if age >= ttl && !refreshing {
		c.mu.Lock()
		if !c.refreshing {
			c.refreshing = true
			go func() {
				all, err := loadAll(context.Background())
				c.mu.Lock()
				defer c.mu.Unlock()
				if err == nil {
					c.items = all
					c.loadedAt = nowFn()
				}
				c.refreshing = false
			}()
		}
		items = c.items
		c.mu.Unlock()
	}

	tenantConfigs := items[tenantID]
	if len(tenantConfigs) == 0 {
		return nil, nil
	}
	config := tenantConfigs[messageType]
	if len(config) == 0 {
		return nil, nil
	}
	return config, nil
}
