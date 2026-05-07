package exampleaggregateonly

import (
	"context"
	"testing"
	"time"

	"work/notification/code/contract"
)

func TestHandlerAggregate(t *testing.T) {
	h := &Handler{}

	result, err := h.Aggregate(context.Background(), &contract.BizAggregateRequest{
		WindowStart: time.Date(2026, 4, 29, 11, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		Filter: &Filter{
			Severity: []string{"high", "critical"},
		},
	})
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if result == nil || result.BizVars == nil {
		t.Fatal("expected aggregate biz vars")
	}
	if result.BizVars["severity_count"] != 2 {
		t.Fatalf("unexpected severity_count: %v", result.BizVars["severity_count"])
	}
}
