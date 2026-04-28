package xdrriskdigest

import (
	"context"
	"testing"

	core "notes/code/aggregate_registry_demo"
)

func TestAggregatorAggregate(t *testing.T) {
	agg := New()

	result, err := agg.Aggregate(context.Background(), &core.BizAggregateRequest{
	})
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if result.MessageType != agg.MessageType() {
		t.Fatalf("unexpected message_type: %s", result.MessageType)
	}
	if result.BizVars == nil {
		t.Fatal("expected biz_vars map")
	}
}

func TestAggregatorImplementsContract(t *testing.T) {
	agg := New()
	if agg.MessageType() != "xdr_risk_digest" {
		t.Fatalf("unexpected message type: %s", agg.MessageType())
	}
}
