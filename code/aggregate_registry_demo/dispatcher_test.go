package aggregate

import (
	"context"
	"errors"
	"testing"

	"notes/code/aggregate_registry_demo/messages"
)

type stubPublisher struct {
	msg *DispatchMessage
	err error
}

func (w *stubPublisher) Publish(_ context.Context, msg *DispatchMessage) error {
	if w.err != nil {
		return w.err
	}
	w.msg = msg
	return nil
}

type stubRealtimeHandler struct {
	decision *RealtimeDecision
	err      error
}

func (h stubRealtimeHandler) MessageType() string { return "realtime_stub" }
func (h stubRealtimeHandler) MustRegister()       {}
func (h stubRealtimeHandler) Aggregate(_ context.Context, _ *BizAggregateRequest) (*messages.BizAggregateResult, error) {
	return &messages.BizAggregateResult{MessageType: h.MessageType(), BizVars: messages.TemplateVars{}}, nil
}
func (h stubRealtimeHandler) Evaluate(_ context.Context, _ *RealtimeRequest) (*RealtimeDecision, error) {
	return h.decision, h.err
}

func TestDispatcherDispatch(t *testing.T) {
	writer := &stubPublisher{}
	dispatcher := &Dispatcher{Publisher: writer}

	err := dispatcher.Dispatch(context.Background(), &DispatchMessage{
		TenantID:    "t_1001",
		MessageType: "xdr_risk_digest",
		BizVars:     messages.TemplateVars{"risk_type": "暴力破解"},
	})
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if writer.msg == nil {
		t.Fatal("expected dispatch message to be published")
	}
}

func TestDispatcherHandleRealtimeMatched(t *testing.T) {
	writer := &stubPublisher{}
	dispatcher := &Dispatcher{Publisher: writer}

	decision, err := dispatcher.HandleRealtime(context.Background(), stubRealtimeHandler{
		decision: &RealtimeDecision{
			Matched: true,
			BizVars: messages.TemplateVars{"risk_type": "暴力破解"},
		},
	}, &RealtimeRequest{TenantID: "t_1001"})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if !decision.Matched {
		t.Fatal("expected matched decision")
	}
	if writer.msg == nil {
		t.Fatal("expected dispatch message to be published")
	}
}

func TestDispatcherHandleRealtimeNotMatched(t *testing.T) {
	writer := &stubPublisher{}
	dispatcher := &Dispatcher{Publisher: writer}

	decision, err := dispatcher.HandleRealtime(context.Background(), stubRealtimeHandler{
		decision: &RealtimeDecision{Matched: false},
	}, &RealtimeRequest{TenantID: "t_1001"})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if decision.Matched {
		t.Fatal("expected unmatched decision")
	}
	if writer.msg != nil {
		t.Fatal("did not expect dispatch message to be published")
	}
}

func TestDispatcherHandleRealtimePublisherError(t *testing.T) {
	expectedErr := errors.New("publish failed")
	dispatcher := &Dispatcher{Publisher: &stubPublisher{err: expectedErr}}

	_, err := dispatcher.HandleRealtime(context.Background(), stubRealtimeHandler{
		decision: &RealtimeDecision{Matched: true, BizVars: messages.TemplateVars{}},
	}, &RealtimeRequest{TenantID: "t_1001"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected publisher error, got %v", err)
	}
}
