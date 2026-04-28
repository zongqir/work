package aggregate

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"notes/code/aggregate_registry_demo/messages"
)

type stubPublisher struct {
	msg *DispatchMessage
}

func (p *stubPublisher) Publish(_ context.Context, msg *DispatchMessage) error {
	p.msg = msg
	return nil
}

type stubSendHandler struct {
	messageType     string
	aggregateCalled bool
	realtimeCalled  bool
}

func (h *stubSendHandler) MessageType() string { return h.messageType }
func (h *stubSendHandler) MustRegister()       {}
func (h *stubSendHandler) Aggregate(_ context.Context, req *BizAggregateRequest) (*messages.BizAggregateResult, error) {
	h.aggregateCalled = true
	return &messages.BizAggregateResult{
		MessageType: h.MessageType(),
		BizVars: messages.TemplateVars{
			"config": string(req.ConfigBody),
		},
	}, nil
}
func (h *stubSendHandler) Evaluate(_ context.Context, req *RealtimeRequest) (*RealtimeDecision, error) {
	h.realtimeCalled = true
	return &RealtimeDecision{
		Matched: true,
		BizVars: messages.TemplateVars{
			"filter": string(req.FilterQuery),
		},
	}, nil
}

func TestSendAggregate(t *testing.T) {
	handler := &stubSendHandler{messageType: "send_test_aggregate"}
	MustRegister(handler)

	publisher := &stubPublisher{}
	dispatcher := &Dispatcher{
		Publisher: publisher,
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_1": {
					"send_test_aggregate": json.RawMessage(`{"enabled":true,"aggregate_filter":{"k":"v"}}`),
				},
			}, nil
		},
	}

	err := dispatcher.SendAggregate(context.Background(), "t_1", "send_test_aggregate", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("SendAggregate failed: %v", err)
	}
	if !handler.aggregateCalled {
		t.Fatal("expected aggregate handler to be called")
	}
	if publisher.msg == nil {
		t.Fatal("expected message to be published")
	}
}

func TestSendRealtime(t *testing.T) {
	handler := &stubSendHandler{messageType: "send_test_realtime"}
	MustRegister(handler)

	publisher := &stubPublisher{}
	dispatcher := &Dispatcher{
		Publisher: publisher,
		LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
			return map[string]map[string]json.RawMessage{
				"t_2": {
					"send_test_realtime": json.RawMessage(`{"enabled":true,"realtime_filter":{"x":"y"}}`),
				},
			}, nil
		},
	}

	err := dispatcher.SendRealtime(context.Background(), "t_2", "send_test_realtime", json.RawMessage(`{"event":1}`))
	if err != nil {
		t.Fatalf("SendRealtime failed: %v", err)
	}
	if !handler.realtimeCalled {
		t.Fatal("expected realtime handler to be called")
	}
	if publisher.msg == nil {
		t.Fatal("expected message to be published")
	}
}
