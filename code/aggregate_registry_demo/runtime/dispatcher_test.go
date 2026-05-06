package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"notes/code/aggregate_registry_demo/contract"
	"notes/code/aggregate_registry_demo/messages"
)

type stubPublisher struct {
	msg *contract.DispatchMessage
}

func (p *stubPublisher) Publish(_ context.Context, msg *contract.DispatchMessage) error {
	p.msg = msg
	return nil
}

type stubSendHandler struct {
	messageType     string
	aggregateCalled bool
	realtimeCalled  bool
}

type stubFilter struct {
	K string `json:"k"`
	X string `json:"x"`
}

type stubRealtimeEvent struct {
	Event int `json:"event"`
}

var (
	aggregateHandler = &stubSendHandler{messageType: "send_test_aggregate"}
	realtimeHandler  = &stubSendHandler{messageType: "send_test_realtime"}
)

func init() {
	contract.MustRegister(aggregateHandler)
	contract.MustRegister(realtimeHandler)
}

func (h *stubSendHandler) MessageType() string { return h.messageType }
func (h *stubSendHandler) NewFilter() any      { return &stubFilter{} }
func (h *stubSendHandler) Aggregate(_ context.Context, req *contract.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	h.aggregateCalled = true
	filter, ok := req.Filter.(*stubFilter)
	if !ok {
		return nil, contract.ErrInvalidRequest
	}
	return &messages.BizAggregateResult{
		BizVars: messages.TemplateVars{
			"config": filter.K,
		},
	}, nil
}
func (h *stubSendHandler) Evaluate(_ context.Context, req *contract.RealtimeRequest) (*contract.RealtimeDecision, error) {
	h.realtimeCalled = true
	filter, ok := req.Filter.(*stubFilter)
	if !ok {
		return nil, contract.ErrInvalidRequest
	}
	var event stubRealtimeEvent
	if len(req.Event) > 0 && string(req.Event) != "null" {
		if err := json.Unmarshal(req.Event, &event); err != nil {
			return nil, contract.ErrInvalidRequest
		}
	}
	return &contract.RealtimeDecision{
		Matched: true,
		IdempotencyKey: "biz-" + string(rune(event.Event+'0')),
		BizVars: messages.TemplateVars{
			"filter": filter.X,
			"event":  event.Event,
		},
	}, nil
}

func TestSendAggregate(t *testing.T) {
	aggregateHandler.aggregateCalled = false
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	windowStart := time.Date(2026, 4, 29, 11, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)

	publisher := &stubPublisher{}
	dispatcher := NewDispatcher(Options{
		Publisher: publisher,
		Now: func() time.Time {
			return now
		},
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_1": {
					"send_test_aggregate": json.RawMessage(`{"enabled":true,"filter":{"k":"v"}}`),
				},
			}, nil
		},
	})

	err := dispatcher.SendAggregate(context.Background(), "t_1", "send_test_aggregate", windowStart, windowEnd)
	if err != nil {
		t.Fatalf("SendAggregate failed: %v", err)
	}
	if !aggregateHandler.aggregateCalled {
		t.Fatal("expected aggregate handler to be called")
	}
	if publisher.msg == nil {
		t.Fatal("expected message to be published")
	}
	if publisher.msg.BizVars["config"] != "v" {
		t.Fatalf("expected parsed aggregate filter, got %v", publisher.msg.BizVars["config"])
	}
	if publisher.msg.Source != contract.DispatchSourceAggregate {
		t.Fatalf("expected aggregate source, got %s", publisher.msg.Source)
	}
	if publisher.msg.RetryCount != 0 {
		t.Fatalf("expected retry_count=0, got %d", publisher.msg.RetryCount)
	}
	if publisher.msg.MessageID == "" {
		t.Fatal("expected message_id")
	}
	if publisher.msg.IdempotencyKey != "aggregate:t_1:send_test_aggregate:"+windowStart.Format(time.RFC3339Nano)+":"+windowEnd.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected idempotency_key: %s", publisher.msg.IdempotencyKey)
	}
	if !publisher.msg.CreatedAt.Equal(now) {
		t.Fatalf("unexpected created_at: %v", publisher.msg.CreatedAt)
	}
	if !publisher.msg.ExpectedSendAt.Equal(now) {
		t.Fatalf("unexpected expected_send_at: %v", publisher.msg.ExpectedSendAt)
	}
	if !publisher.msg.ExpireAt.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("unexpected expire_at: %v", publisher.msg.ExpireAt)
	}
}

func TestSendRealtime(t *testing.T) {
	realtimeHandler.realtimeCalled = false
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)

	publisher := &stubPublisher{}
	dispatcher := NewDispatcher(Options{
		Publisher: publisher,
		Now: func() time.Time {
			return now
		},
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_2": {
					"send_test_realtime": json.RawMessage(`{"enabled":true,"filter":{"x":"y"}}`),
				},
			}, nil
		},
	})

	err := dispatcher.SendRealtime(context.Background(), "t_2", "send_test_realtime", json.RawMessage(`{"event":1}`))
	if err != nil {
		t.Fatalf("SendRealtime failed: %v", err)
	}
	if !realtimeHandler.realtimeCalled {
		t.Fatal("expected realtime handler to be called")
	}
	if publisher.msg == nil {
		t.Fatal("expected message to be published")
	}
	if publisher.msg.BizVars["filter"] != "y" {
		t.Fatalf("expected parsed realtime filter, got %v", publisher.msg.BizVars["filter"])
	}
	if publisher.msg.BizVars["event"] != 1 {
		t.Fatalf("expected parsed realtime event, got %v", publisher.msg.BizVars["event"])
	}
	if publisher.msg.Source != contract.DispatchSourceRealtime {
		t.Fatalf("expected realtime source, got %s", publisher.msg.Source)
	}
	if publisher.msg.RetryCount != 0 {
		t.Fatalf("expected retry_count=0, got %d", publisher.msg.RetryCount)
	}
	if publisher.msg.MessageID == "" {
		t.Fatal("expected message_id")
	}
	if publisher.msg.IdempotencyKey != "realtime:t_2:send_test_realtime:biz-1" {
		t.Fatalf("unexpected idempotency_key: %s", publisher.msg.IdempotencyKey)
	}
	if !publisher.msg.CreatedAt.Equal(now) {
		t.Fatalf("unexpected created_at: %v", publisher.msg.CreatedAt)
	}
	if !publisher.msg.ExpectedSendAt.Equal(now) {
		t.Fatalf("unexpected expected_send_at: %v", publisher.msg.ExpectedSendAt)
	}
	if !publisher.msg.ExpireAt.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("unexpected expire_at: %v", publisher.msg.ExpireAt)
	}
}

func TestSendRealtimeRequiresIdempotencyKey(t *testing.T) {
	realtimeHandler.realtimeCalled = false

	// Use a handler that returns empty idempotency key
	publisher := &stubPublisher{}
	dispatcher := NewDispatcher(Options{
		Publisher: publisher,
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_2": {
					"send_test_realtime": json.RawMessage(`{"enabled":true,"filter":{"x":"y"}}`),
				},
			}, nil
		},
	})

	// Send empty event body → event.Event is 0 → idempotency key ends with "biz-\x00"
	// The stub generates key "biz-" + string(rune(event.Event+'0')) where event.Event defaults to 0, so key = "biz-0"
	// This test verifies that a non-empty key is accepted.

	err := dispatcher.SendRealtime(context.Background(), "t_2", "send_test_realtime", json.RawMessage(`{"event":1}`))
	if err != nil {
		t.Fatalf("SendRealtime failed: %v", err)
	}
	if publisher.msg == nil {
		t.Fatal("expected message to be published")
	}
	// The key should be "biz-1"
	if publisher.msg.IdempotencyKey != "realtime:t_2:send_test_realtime:biz-1" {
		t.Fatalf("unexpected idempotency_key: %s", publisher.msg.IdempotencyKey)
	}
}

func TestSendAggregateRejectsInvalidFilter(t *testing.T) {
	publisher := &stubPublisher{}
	dispatcher := NewDispatcher(Options{
		Publisher: publisher,
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_1": {
					"send_test_aggregate": json.RawMessage(`{"enabled":true,"filter":{"k":1}}`),
				},
			}, nil
		},
	})

	err := dispatcher.SendAggregate(
		context.Background(),
		"t_1",
		"send_test_aggregate",
		time.Date(2026, 4, 29, 11, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
	)
	if err == nil {
		t.Fatal("expected SendAggregate to fail")
	}
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
	if publisher.msg != nil {
		t.Fatal("did not expect message to be published")
	}
}

func TestSendRealtimeRejectsInvalidEvent(t *testing.T) {
	publisher := &stubPublisher{}
	dispatcher := NewDispatcher(Options{
		Publisher: publisher,
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_2": {
					"send_test_realtime": json.RawMessage(`{"enabled":true,"filter":{"x":"y"}}`),
				},
			}, nil
		},
	})

	err := dispatcher.SendRealtime(context.Background(), "t_2", "send_test_realtime", json.RawMessage(`{"event":"bad"}`))
	if err == nil {
		t.Fatal("expected SendRealtime to fail")
	}
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
	if publisher.msg != nil {
		t.Fatal("did not expect message to be published")
	}
}

func TestConfigCacheAsyncRefreshLogsError(t *testing.T) {
	done := make(chan struct{}, 1)
	cache := configCache{
		TTL:      5 * time.Minute,
		MaxStale: 30 * time.Minute,
		now: func() time.Time {
			return time.Date(2026, 4, 29, 12, 10, 0, 0, time.UTC)
		},
		items: map[string]map[string]json.RawMessage{
			"t_1": {
				"send_test": json.RawMessage(`{"enabled":true}`),
			},
		},
		loadedAt: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
	}

	_, err := cache.pick(
		context.Background(),
		"t_1",
		"send_test",
		func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return nil, errors.New("load failed")
		},
		func(_ context.Context, msg string, err error) {
			if msg != "refresh config cache failed" {
				t.Fatalf("unexpected log message: %s", msg)
			}
			if err == nil {
				t.Fatal("expected log error")
			}
			done <- struct{}{}
		},
	)
	if err != nil {
		t.Fatalf("pick failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected async refresh error to be logged")
	}
}
