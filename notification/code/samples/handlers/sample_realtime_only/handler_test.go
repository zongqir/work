package samplerealtimeonly

import (
	"context"
	"testing"

	"work/notification/code/pkg/notification"
)

func TestHandlerEvaluateMatched(t *testing.T) {
	h := &Handler{}

	result, err := h.Evaluate(context.Background(), &notification.RealtimeRequest{
		Filter: &Filter{
			Severity: []string{"high"},
		},
		Event: &Event{
			EventID:  "evt-1",
			Severity: "high",
			Title:    "high risk",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if result == nil || !result.Matched {
		t.Fatal("expected matched realtime result")
	}
	if result.IdempotencyKey != "evt-1" {
		t.Fatalf("unexpected idempotency key: %s", result.IdempotencyKey)
	}
}

func TestHandlerEvaluateUnmatched(t *testing.T) {
	h := &Handler{}

	result, err := h.Evaluate(context.Background(), &notification.RealtimeRequest{
		Filter: &Filter{
			Severity: []string{"critical"},
		},
		Event: &Event{
			EventID:  "evt-1",
			Severity: "low",
			Title:    "low risk",
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected realtime result")
	}
	if result.Matched {
		t.Fatal("expected unmatched realtime result")
	}
}
