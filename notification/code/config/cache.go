package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"work/notification/code/contract"
)

const defaultTTL = 5 * time.Minute

type Cache struct {
	TTL            time.Duration
	MaxStale       time.Duration
	RefreshTimeout time.Duration
	Now            func() time.Time
	items          map[string]map[string]json.RawMessage
	loadedAt       time.Time
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

	if raw := c.lookup(tenantID, messageType); len(raw) > 0 {
		return raw, nil
	}

	all, err := loadAll(ctx)
	if err != nil {
		if logError != nil {
			logError(ctx, "load config cache failed", err)
		}
		return nil, err
	}

	c.items = all
	c.loadedAt = c.now()
	return pickFrom(all, tenantID, messageType), nil
}

func (c *Cache) lookup(tenantID, messageType string) json.RawMessage {
	if c.items == nil || c.loadedAt.IsZero() {
		return nil
	}
	if c.age() >= c.ttl() {
		return nil
	}
	return pickFrom(c.items, tenantID, messageType)
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

func (c *Cache) age() time.Duration {
	if c.loadedAt.IsZero() {
		return 0
	}
	return c.now().Sub(c.loadedAt)
}
