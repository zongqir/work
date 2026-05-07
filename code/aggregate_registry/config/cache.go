package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"notes/code/aggregate_registry/contract"
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

	c.mu.Lock()
	items := c.items
	loadedAt := c.loadedAt
	age := now.Sub(loadedAt)
	needsReload := items == nil || loadedAt.IsZero() || age >= maxStale
	shouldRefresh := !needsReload && age >= ttl && !c.refreshing
	if shouldRefresh {
		c.refreshing = true
	}
	c.mu.Unlock()

	if needsReload {
		all, err := loadAll(ctx)
		if err != nil {
			return nil, err
		}
		c.store(all)
		items = all
	} else if shouldRefresh {
		go c.refresh(loadAll, logError)
	}

	return pickFrom(items, tenantID, messageType), nil
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

