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

var (
	aggregateHandler = &stubSendHandler{messageType: "send_test_aggregate"}
	realtimeHandler  = &stubSendHandler{messageType: "send_test_realtime"}
)

func init() {
	contract.MustRegister(aggregateHandler)
	contract.MustRegister(realtimeHandler)
}

func (h *stubSendHandler) MessageType() string { return h.messageType }
func (h *stubSendHandler) MustRegister()       {}
func (h *stubSendHandler) Aggregate(_ context.Context, req *contract.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	h.aggregateCalled = true
	return &messages.BizAggregateResult{
		BizVars: messages.TemplateVars{
			"config": string(req.ConfigBody),
		},
	}, nil
}
func (h *stubSendHandler) Evaluate(_ context.Context, req *contract.RealtimeRequest) (*contract.RealtimeDecision, error) {
	h.realtimeCalled = true
	return &contract.RealtimeDecision{
		Matched: true,
		BizVars: messages.TemplateVars{
			"filter": string(req.FilterQuery),
		},
	}, nil
}

func TestSendAggregate(t *testing.T) {
	aggregateHandler.aggregateCalled = false

	publisher := &stubPublisher{}
	dispatcher := NewDispatcher(Options{
		Publisher: publisher,
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_1": {
					"send_test_aggregate": json.RawMessage(`{"enabled":true,"aggregate_filter":{"k":"v"}}`),
				},
			}, nil
		},
	})

	err := dispatcher.SendAggregate(context.Background(), "t_1", "send_test_aggregate", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("SendAggregate failed: %v", err)
	}
	if !aggregateHandler.aggregateCalled {
		t.Fatal("expected aggregate handler to be called")
	}
	if publisher.msg == nil {
		t.Fatal("expected message to be published")
	}
}

func TestSendRealtime(t *testing.T) {
	realtimeHandler.realtimeCalled = false

	publisher := &stubPublisher{}
	dispatcher := NewDispatcher(Options{
		Publisher: publisher,
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_2": {
					"send_test_realtime": json.RawMessage(`{"enabled":true,"realtime_filter":{"x":"y"}}`),
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
