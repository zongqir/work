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
	LoadAll   func(ctx context.Context) (map[string]json.RawMessage, error)

	cache configCache
}

func (d *Dispatcher) Send(ctx context.Context, tenantID, messageType string, eventBody json.RawMessage) error {
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

	configs, err := d.cache.pick(ctx, []string{tenantID}, d.LoadAll)
	if err != nil {
		return err
	}
	configBody := configs[tenantID]
	if len(configBody) == 0 {
		return nil
	}

	var config struct {
		Enabled     bool            `json:"enabled"`
		FilterQuery json.RawMessage `json:"filter_query"`
	}
	if err := json.Unmarshal(configBody, &config); err != nil {
		return err
	}
	if !config.Enabled {
		return nil
	}

	decision, err := d.sendWithHandler(ctx, handler, &RealtimeRequest{
		TenantID:    tenantID,
		FilterQuery: config.FilterQuery,
		EventBody:   eventBody,
	})
	if err != nil {
		return err
	}
	if decision == nil {
		return nil
	}
	return nil
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

func (d *Dispatcher) sendWithHandler(ctx context.Context, handler Handler, req *RealtimeRequest) (*RealtimeDecision, error) {
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

	items      map[string]json.RawMessage
	loadedAt   time.Time
	refreshing bool
}

func (c *configCache) pick(ctx context.Context, tenantIDs []string, loadAll func(context.Context) (map[string]json.RawMessage, error)) (map[string]json.RawMessage, error) {
	if c == nil {
		return nil, nil
	}
	if len(tenantIDs) == 0 {
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

	selected := make(map[string]json.RawMessage, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		if config := items[tenantID]; config != nil {
			selected[tenantID] = config
		}
	}
	if len(selected) == 0 {
		return nil, nil
	}
	return selected, nil
}
