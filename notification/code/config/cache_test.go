package config

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestCacheLoadsAndCaches(t *testing.T) {
	var calls int
	cache := Cache{
		TTL: 5 * time.Minute,
		Now: func() time.Time {
			return time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
		},
	}
	loadAll := func(context.Context) (map[string]map[string]json.RawMessage, error) {
		calls++
		return map[string]map[string]json.RawMessage{
			"t_1": {
				"send_test": json.RawMessage(`{"realtime_enabled":true}`),
			},
		}, nil
	}

	raw, err := cache.Pick(context.Background(), "t_1", "send_test", loadAll, nil)
	if err != nil {
		t.Fatalf("pick failed: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected raw config")
	}

	raw, err = cache.Pick(context.Background(), "t_1", "send_test", loadAll, nil)
	if err != nil {
		t.Fatalf("pick failed: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected cached raw config")
	}
	if calls != 1 {
		t.Fatalf("expected one load call, got %d", calls)
	}
}

func TestCacheReloadsAfterTTL(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	cache := Cache{
		TTL: 5 * time.Minute,
		Now: func() time.Time {
			return now
		},
		items: map[string]map[string]json.RawMessage{
			"t_1": {
				"send_test": json.RawMessage(`{"realtime_enabled":true}`),
			},
		},
		loadedAt: now.Add(-10 * time.Minute),
	}

	calls := 0
	loadAll := func(context.Context) (map[string]map[string]json.RawMessage, error) {
		calls++
		return map[string]map[string]json.RawMessage{
			"t_1": {
				"send_test": json.RawMessage(`{"realtime_enabled":false}`),
			},
		}, nil
	}

	raw, err := cache.Pick(context.Background(), "t_1", "send_test", loadAll, nil)
	if err != nil {
		t.Fatalf("pick failed: %v", err)
	}
	if string(raw) != `{"realtime_enabled":false}` {
		t.Fatalf("unexpected raw config: %s", raw)
	}
	if calls != 1 {
		t.Fatalf("expected reload, got %d calls", calls)
	}
}

func TestCacheReturnsLoadError(t *testing.T) {
	cache := Cache{}
	_, err := cache.Pick(context.Background(), "t_1", "send_test", func(context.Context) (map[string]map[string]json.RawMessage, error) {
		return nil, errors.New("load failed")
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
