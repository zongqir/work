package xdrriskdigest

import (
	"context"
	"testing"
	"time"

	core "notes/code/aggregate_registry_demo"
)

func TestAggregatorAggregate(t *testing.T) {
	agg := &Aggregator{}

	result, err := agg.Aggregate(context.Background(), &core.BizAggregateRequest{
		TenantID:    "t_1001",
		WindowStart: time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC),
		ConfigBody:  []byte(`{"severity":["high","critical"],"sample_limit":2}`),
	})
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if result.MessageType != MessageType {
		t.Fatalf("unexpected message_type: %s", result.MessageType)
	}
	if got := result.BizVars["total_count"]; got != "23" {
		t.Fatalf("unexpected total_count: %v", got)
	}
	examples, ok := result.BizVars["examples"].([]map[string]string)
	if !ok {
		t.Fatalf("examples has unexpected type: %T", result.BizVars["examples"])
	}
	if len(examples) != 2 {
		t.Fatalf("unexpected example size: %d", len(examples))
	}
}

func TestAggregatorAggregateInvalidRequest(t *testing.T) {
	agg := &Aggregator{}
	_, err := agg.Aggregate(context.Background(), &core.BizAggregateRequest{
		TenantID:    "",
		WindowStart: time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected invalid request error")
	}
}
