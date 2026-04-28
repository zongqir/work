package aggregate

import (
	"context"
	"errors"
	"testing"

	"notes/code/aggregate_registry_demo/messages"
)

type stubPendingWriter struct {
	msg *PendingMessage
	err error
}

func (w *stubPendingWriter) WritePending(_ context.Context, msg *PendingMessage) error {
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

func TestRealtimeSDKHandleMatched(t *testing.T) {
	writer := &stubPendingWriter{}
	sdk := &RealtimeSDK{Writer: writer}

	decision, err := sdk.Handle(context.Background(), stubRealtimeHandler{
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
		t.Fatal("expected pending message to be written")
	}
}

func TestRealtimeSDKHandleNotMatched(t *testing.T) {
	writer := &stubPendingWriter{}
	sdk := &RealtimeSDK{Writer: writer}

	decision, err := sdk.Handle(context.Background(), stubRealtimeHandler{
		decision: &RealtimeDecision{Matched: false},
	}, &RealtimeRequest{TenantID: "t_1001"})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if decision.Matched {
		t.Fatal("expected unmatched decision")
	}
	if writer.msg != nil {
		t.Fatal("did not expect pending message to be written")
	}
}

func TestRealtimeSDKHandleWriterError(t *testing.T) {
	expectedErr := errors.New("write failed")
	sdk := &RealtimeSDK{Writer: &stubPendingWriter{err: expectedErr}}

	_, err := sdk.Handle(context.Background(), stubRealtimeHandler{
		decision: &RealtimeDecision{Matched: true, BizVars: messages.TemplateVars{}},
	}, &RealtimeRequest{TenantID: "t_1001"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected writer error, got %v", err)
	}
}
