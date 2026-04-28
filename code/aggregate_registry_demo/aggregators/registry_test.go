package aggregators

import (
	"context"
	"errors"
	"testing"

	core "notes/code/aggregate_registry_demo"
	"notes/code/aggregate_registry_demo/messages"
)

type stubAggregator struct{}

func (stubAggregator) Aggregate(_ context.Context, _ *core.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	return &messages.BizAggregateResult{MessageType: "stub"}, nil
}

func (stubAggregator) MessageType() string {
	return "stub"
}

func TestResolveNotFound(t *testing.T) {
	_, err := Resolve("missing")
	if !errors.Is(err, core.ErrAggregatorNotFound) {
		t.Fatalf("expected ErrAggregatorNotFound, got %v", err)
	}
}

func TestMustRegisterAndResolve(t *testing.T) {
	const messageType = "stub_for_test"
	testAgg := stubAggregatorWithType(messageType)
	MustRegister(testAgg)

	aggregator, err := Resolve(messageType)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	result, err := aggregator.Aggregate(context.Background(), &core.BizAggregateRequest{})
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if result.MessageType != "stub" {
		t.Fatalf("unexpected message type: %s", result.MessageType)
	}
}

type stubAggregatorWithType string

func (s stubAggregatorWithType) Aggregate(_ context.Context, _ *core.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	return &messages.BizAggregateResult{MessageType: "stub"}, nil
}

func (s stubAggregatorWithType) MessageType() string {
	return string(s)
}
