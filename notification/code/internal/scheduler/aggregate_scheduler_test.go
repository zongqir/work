package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type stubAggregateSender struct {
	tenantID    string
	messageType string
	windowStart time.Time
	windowEnd   time.Time
	called      bool
	sent        bool
	err         error
}

func (s *stubAggregateSender) SendAggregate(_ context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) (bool, error) {
	s.called = true
	s.tenantID = tenantID
	s.messageType = messageType
	s.windowStart = windowStart
	s.windowEnd = windowEnd
	if s.err != nil {
		return false, s.err
	}
	return s.sent, nil
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
	sender := &stubAggregateSender{sent: true}
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
	sender := &stubAggregateSender{sent: true}
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
	sender := &stubAggregateSender{sent: true}
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

func TestAggregateSchedulerDoesNotSaveWatermarkWhenSendSkipped(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 7, 0, 0, time.UTC)
	sender := &stubAggregateSender{sent: false}
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
	if len(store.values) != 0 {
		t.Fatalf("did not expect watermark to be saved: %+v", store.values)
	}
}

func TestAggregateSchedulerRunReturnsTickErrorWithoutLogger(t *testing.T) {
	target := errors.New("load failed")
	scheduler := &AggregateScheduler{
		Sender:         &stubAggregateSender{},
		WatermarkStore: &stubAggregateWatermarkStore{},
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return nil, target
		},
	}

	err := scheduler.Run(context.Background())
	if !errors.Is(err, target) {
		t.Fatalf("expected load error, got %v", err)
	}
}
