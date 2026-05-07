package scheduler

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type stubAggregateSender struct {
	tenantID    string
	messageType string
	windowStart time.Time
	windowEnd   time.Time
	called      bool
}

func (s *stubAggregateSender) SendAggregate(_ context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) error {
	s.called = true
	s.tenantID = tenantID
	s.messageType = messageType
	s.windowStart = windowStart
	s.windowEnd = windowEnd
	return nil
}

type stubAggregateWatermarkStore struct {
	values map[string]time.Time
}

func (s *stubAggregateWatermarkStore) LastWindowEnd(_ context.Context, tenantID, messageType string) (time.Time, error) {
	return s.values[tenantID+":"+messageType], nil
}

func (s *stubAggregateWatermarkStore) SaveWindowEnd(_ context.Context, tenantID, messageType string, windowEnd time.Time) error {
	if s.values == nil {
		s.values = map[string]time.Time{}
	}
	s.values[tenantID+":"+messageType] = windowEnd
	return nil
}

func TestAggregateSchedulerTickFirstWindow(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 7, 0, 0, time.UTC)
	sender := &stubAggregateSender{}
	store := &stubAggregateWatermarkStore{}
	scheduler := &AggregateScheduler{
		Sender:         sender,
		WatermarkStore: store,
		Now: func() time.Time {
			return now
		},
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_1": {
					"sample_both": json.RawMessage(`{"aggregate_enabled":true,"aggregate_period_minutes":5,"filter":{"k":"v"}}`),
				},
			}, nil
		},
	}

	err := scheduler.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}
	if !sender.called {
		t.Fatal("expected SendAggregate to be called")
	}
	if !sender.windowStart.Equal(time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected window_start: %v", sender.windowStart)
	}
	if !sender.windowEnd.Equal(time.Date(2026, 4, 29, 12, 5, 0, 0, time.UTC)) {
		t.Fatalf("unexpected window_end: %v", sender.windowEnd)
	}
	if got := store.values["t_1:sample_both"]; !got.Equal(sender.windowEnd) {
		t.Fatalf("unexpected saved watermark: %v", got)
	}
}

func TestAggregateSchedulerTickSkipWhenNotDue(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 7, 0, 0, time.UTC)
	sender := &stubAggregateSender{}
	store := &stubAggregateWatermarkStore{
		values: map[string]time.Time{
			"t_1:sample_both": time.Date(2026, 4, 29, 12, 10, 0, 0, time.UTC),
		},
	}
	scheduler := &AggregateScheduler{
		Sender:         sender,
		WatermarkStore: store,
		Now: func() time.Time {
			return now
		},
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_1": {
					"sample_both": json.RawMessage(`{"aggregate_enabled":true,"aggregate_period_minutes":5}`),
				},
			}, nil
		},
	}

	err := scheduler.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}
	if sender.called {
		t.Fatal("did not expect SendAggregate to be called")
	}
}

func TestAggregateSchedulerTickNextWindowFromWatermark(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 16, 0, 0, time.UTC)
	sender := &stubAggregateSender{}
	store := &stubAggregateWatermarkStore{
		values: map[string]time.Time{
			"t_1:sample_both": time.Date(2026, 4, 29, 12, 5, 0, 0, time.UTC),
		},
	}
	scheduler := &AggregateScheduler{
		Sender:         sender,
		WatermarkStore: store,
		Now: func() time.Time {
			return now
		},
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_1": {
					"sample_both": json.RawMessage(`{"aggregate_enabled":true,"aggregate_period_minutes":5}`),
				},
			}, nil
		},
	}

	err := scheduler.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}
	if !sender.called {
		t.Fatal("expected SendAggregate to be called")
	}
	if !sender.windowStart.Equal(time.Date(2026, 4, 29, 12, 5, 0, 0, time.UTC)) {
		t.Fatalf("unexpected window_start: %v", sender.windowStart)
	}
	if !sender.windowEnd.Equal(time.Date(2026, 4, 29, 12, 10, 0, 0, time.UTC)) {
		t.Fatalf("unexpected window_end: %v", sender.windowEnd)
	}
}
