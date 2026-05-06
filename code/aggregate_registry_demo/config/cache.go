package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"notes/code/aggregate_registry_demo/contract"
)

type Cache struct {
	TTL            time.Duration
	MaxStale       time.Duration
	RefreshTimeout time.Duration

	Now func() time.Time
	mu  sync.RWMutex

	items      map[string]map[string]json.RawMessage
	loadedAt   time.Time
	refreshing bool
}

func (c *Cache) Pick(
	ctx context.Context,
	tenantID, messageType string,
	loadAll func(context.Context) (map[string]map[string]json.RawMessage, error),
	logError func(context.Context, string, error),
) (json.RawMessage, error) {
	if c == nil {
		return nil, nil
	}
	if tenantID == "" || messageType == "" {
		return nil, nil
	}
	if loadAll == nil {
		return nil, fmt.Errorf("%w: load_all is required", contract.ErrInvalidRequest)
	}

	nowFn := c.Now
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
	refreshTimeout := c.RefreshTimeout
	if refreshTimeout <= 0 {
		refreshTimeout = 10 * time.Second
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
				refreshCtx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
				defer cancel()

				all, err := loadAll(refreshCtx)
				c.mu.Lock()
				defer c.mu.Unlock()
				if err == nil {
					c.items = all
					c.loadedAt = nowFn()
				} else if logError != nil {
					logError(refreshCtx, "refresh config cache failed", err)
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
	cfg := tenantConfigs[messageType]
	if len(cfg) == 0 {
		return nil, nil
	}
	return cfg, nil
}
