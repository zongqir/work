package config

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
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
				"send_test": json.RawMessage(`{"realtime_enabled":true}`),
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

func TestCacheSharesConcurrentColdReload(t *testing.T) {
	cache := Cache{
		TTL:      5 * time.Minute,
		MaxStale: 30 * time.Minute,
		Now: func() time.Time {
			return time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
		},
	}
	release := make(chan struct{})
	var calls int32

	loadAll := func(context.Context) (map[string]map[string]json.RawMessage, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return map[string]map[string]json.RawMessage{
			"t_1": {
				"send_test": json.RawMessage(`{"realtime_enabled":true}`),
			},
		}, nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw, err := cache.Pick(context.Background(), "t_1", "send_test", loadAll, nil)
			if err != nil {
				errs <- err
				return
			}
			if len(raw) == 0 {
				errs <- errors.New("expected cached config")
			}
		}()
	}

	deadline := time.After(time.Second)
	for atomic.LoadInt32(&calls) == 0 {
		select {
		case <-deadline:
			t.Fatal("expected load_all to be called")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected one load_all call, got %d", calls)
	}
}
