package config

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestCacheAsyncRefreshUsesTimeout(t *testing.T) {
	done := make(chan error, 1)
	cache := Cache{
		TTL:            5 * time.Minute,
		MaxStale:       30 * time.Minute,
		RefreshTimeout: 20 * time.Millisecond,
		Now: func() time.Time {
			return time.Date(2026, 4, 29, 12, 10, 0, 0, time.UTC)
		},
		items: map[string]map[string]json.RawMessage{
			"t_1": {
				"send_test": json.RawMessage(`{"enabled":true}`),
			},
		},
		loadedAt: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
	}

	_, err := cache.Pick(
		context.Background(),
		"t_1",
		"send_test",
		func(ctx context.Context) (map[string]map[string]json.RawMessage, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
		func(_ context.Context, _ string, err error) {
			done <- err
		},
	)
	if err != nil {
		t.Fatalf("pick failed: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context deadline exceeded, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("expected async refresh timeout to be logged")
	}
}
