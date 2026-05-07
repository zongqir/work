package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"work/notification/code/contract"
)

const (
	defaultTTL            = 5 * time.Minute
	defaultMaxStale       = 30 * time.Minute
	defaultRefreshTimeout = 10 * time.Second
)

type Cache struct {
	TTL            time.Duration
	MaxStale       time.Duration
	RefreshTimeout time.Duration

	Now func() time.Time
	mu  sync.Mutex

	items      map[string]map[string]json.RawMessage
	loadedAt   time.Time
	refreshing bool
	loading    bool
	loadDone   chan struct{}
	loadErr    error
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

	now := c.now()
	ttl := c.ttl()
	maxStale := c.maxStale()

	for {
		c.mu.Lock()
		items := c.items
		loadedAt := c.loadedAt
		age := now.Sub(loadedAt)
		needsReload := items == nil || loadedAt.IsZero() || age >= maxStale
		shouldRefresh := !needsReload && age >= ttl && !c.refreshing
		if needsReload && c.loading {
			done := c.loadDone
			c.mu.Unlock()

			select {
			case <-done:
				c.mu.Lock()
				err := c.loadErr
				c.mu.Unlock()
				if err != nil {
					return nil, err
				}
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		if needsReload {
			done := make(chan struct{})
			c.loading = true
			c.loadDone = done
			c.loadErr = nil
			c.mu.Unlock()

			all, err := loadAll(ctx)
			c.finishLoad(all, err, done)
			if err != nil {
				return nil, err
			}
			return pickFrom(all, tenantID, messageType), nil
		}

		if shouldRefresh {
			c.refreshing = true
		}
		c.mu.Unlock()

		if shouldRefresh {
			go c.refresh(loadAll, logError)
		}
		return pickFrom(items, tenantID, messageType), nil
	}
}

func (c *Cache) refresh(
	loadAll func(context.Context) (map[string]map[string]json.RawMessage, error),
	logError func(context.Context, string, error),
) {
	refreshCtx, cancel := context.WithTimeout(context.Background(), c.refreshTimeout())
	defer cancel()

	all, err := loadAll(refreshCtx)
	if err != nil {
		c.mu.Lock()
		c.refreshing = false
		c.mu.Unlock()
		if logError != nil {
			logError(refreshCtx, "refresh config cache failed", err)
		}
		return
	}

	c.store(all)
}

func (c *Cache) store(items map[string]map[string]json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = items
	c.loadedAt = c.now()
	c.refreshing = false
}

func (c *Cache) finishLoad(items map[string]map[string]json.RawMessage, err error, done chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		c.items = items
		c.loadedAt = c.now()
	}
	c.loading = false
	c.loadErr = err
	close(done)
}

func pickFrom(items map[string]map[string]json.RawMessage, tenantID, messageType string) json.RawMessage {
	tenantConfigs := items[tenantID]
	if len(tenantConfigs) == 0 {
		return nil
	}
	cfg := tenantConfigs[messageType]
	if len(cfg) == 0 {
		return nil
	}
	return cfg
}

func (c *Cache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Cache) ttl() time.Duration {
	if c.TTL > 0 {
		return c.TTL
	}
	return defaultTTL
}

func (c *Cache) maxStale() time.Duration {
	if c.MaxStale > 0 {
		return c.MaxStale
	}
	return defaultMaxStale
}

func (c *Cache) refreshTimeout() time.Duration {
	if c.RefreshTimeout > 0 {
		return c.RefreshTimeout
	}
	return defaultRefreshTimeout
}
