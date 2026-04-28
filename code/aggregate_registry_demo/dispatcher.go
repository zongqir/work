package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
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
}

func (d *Dispatcher) Dispatch(ctx context.Context, msg *DispatchMessage) error {
	if d == nil || d.Publisher == nil {
		return fmt.Errorf("%w: message publisher is required", ErrInvalidRequest)
	}
	if msg == nil {
		return fmt.Errorf("%w: dispatch message is nil", ErrInvalidRequest)
	}

	return d.Publisher.Publish(ctx, msg)
}

func (d *Dispatcher) HandleRealtime(ctx context.Context, handler Handler, req *RealtimeRequest) (*RealtimeDecision, error) {
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

	if err := d.Dispatch(ctx, &DispatchMessage{
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
	TTL time.Duration

	now func() time.Time
	mu  sync.RWMutex

	items     map[string]json.RawMessage
	expiresAt time.Time
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
	expiresAt := c.expiresAt
	c.mu.RUnlock()

	if items == nil || !now.Before(expiresAt) {
		all, err := loadAll(ctx)
		if err != nil {
			return nil, err
		}

		ttl := c.TTL
		if ttl <= 0 {
			ttl = 5 * time.Minute
		}

		c.mu.Lock()
		c.items = all
		c.expiresAt = nowFn().Add(ttl)
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
