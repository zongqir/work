package config

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
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

func TestCacheRefreshesInBackgroundAfterTTL(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	calls := 0
	cache := Cache{
		TTL:      5 * time.Minute,
		MaxStale: 30 * time.Minute,
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
	loadAll := func(context.Context) (map[string]map[string]json.RawMessage, error) {
		calls++
		once.Do(func() { close(started) })
		<-release
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
	if string(raw) != `{"realtime_enabled":true}` {
		t.Fatalf("unexpected stale raw config: %s", raw)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected background refresh to start")
	}

	close(release)

	deadline := time.Now().Add(time.Second)
	for {
		raw, err = cache.Pick(context.Background(), "t_1", "send_test", loadAll, nil)
		if err != nil {
			t.Fatalf("pick failed: %v", err)
		}
		if string(raw) == `{"realtime_enabled":false}` {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected refreshed raw config")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if calls != 1 {
		t.Fatalf("expected one background refresh call, got %d", calls)
	}
}

func TestCacheReloadsSynchronouslyAfterMaxStale(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	calls := 0
	cache := Cache{
		TTL:      5 * time.Minute,
		MaxStale: 30 * time.Minute,
		Now: func() time.Time {
			return now
		},
		items: map[string]map[string]json.RawMessage{
			"t_1": {
				"send_test": json.RawMessage(`{"realtime_enabled":true}`),
			},
		},
		loadedAt: now.Add(-31 * time.Minute),
	}
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
		t.Fatalf("expected sync reload, got %d calls", calls)
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
