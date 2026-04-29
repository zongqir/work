package xdrriskdigest

import (
	"context"
	"testing"

	"notes/code/aggregate_registry_demo/contract"
)

func TestHandlerAggregate(t *testing.T) {
	h := New()

	result, err := h.Aggregate(context.Background(), &contract.BizAggregateRequest{})
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if result.BizVars == nil {
		t.Fatal("expected biz_vars map")
	}
}

func TestHandlerEvaluate(t *testing.T) {
	h := New()

	decision, err := h.Evaluate(context.Background(), &contract.RealtimeRequest{})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !decision.Matched {
		t.Fatal("expected matched decision")
	}
	if decision.BizVars == nil {
		t.Fatal("expected biz_vars map")
	}
}
