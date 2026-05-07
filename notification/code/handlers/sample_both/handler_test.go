package sampleboth

import (
	"context"
	"testing"

	"work/notification/code/contract"
)

func TestHandlerAggregate(t *testing.T) {
	h := &Handler{}

	result, err := h.Aggregate(context.Background(), &contract.BizAggregateRequest{})
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if result.BizVars == nil {
		t.Fatal("expected biz_vars map")
	}
}

func TestHandlerEvaluate(t *testing.T) {
	h := &Handler{}

	decision, err := h.Evaluate(context.Background(), &contract.RealtimeRequest{
		Event: &RealtimeEvent{EventID: "evt-1"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !decision.Matched {
		t.Fatal("expected matched decision")
	}
	if decision.BizVars == nil {
		t.Fatal("expected biz_vars map")
	}
	if decision.IdempotencyKey != "evt-1" {
		t.Fatalf("expected idempotency key evt-1, got %s", decision.IdempotencyKey)
	}
}
