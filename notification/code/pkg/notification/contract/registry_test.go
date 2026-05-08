package contract

import (
	"context"
	"errors"
	"testing"
)

type stubHandler struct{}

func resetRegistryForTest() {
	registeredImplementations = map[string]Registration{}
}

func (stubHandler) MessageType() string { return "stub" }
func (stubHandler) NewFilter() any      { return nil }
func (stubHandler) Aggregate(_ context.Context, _ *BizAggregateRequest) (*BizAggregateResult, error) {
	return &BizAggregateResult{BizVars: TemplateVars{}}, nil
}
func (stubHandler) Evaluate(_ context.Context, _ *RealtimeRequest) (*RealtimeResult, error) {
	return &RealtimeResult{Matched: false, IdempotencyKey: ""}, nil
}

type stubSpec string

func (s stubSpec) MessageType() string { return string(s) }
func (s stubSpec) NewFilter() any      { return nil }

type stubRealtimeOnly struct{}

func (stubRealtimeOnly) Evaluate(_ context.Context, _ *RealtimeRequest) (*RealtimeResult, error) {
	return &RealtimeResult{Matched: true, IdempotencyKey: "biz"}, nil
}

func TestResolveNotFound(t *testing.T) {
	resetRegistryForTest()

	_, err := Resolve("missing")
	if !errors.Is(err, ErrHandlerNotFound) {
		t.Fatalf("expected ErrHandlerNotFound, got %v", err)
	}
}

func TestMustRegisterAndResolve(t *testing.T) {
	resetRegistryForTest()

	handler := stubHandlerWithType("stub_for_test")
	MustRegister(handler)

	resolved, err := Resolve(handler.MessageType())
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	result, err := resolved.Aggregate(context.Background(), &BizAggregateRequest{})
	if err != nil {
		t.Fatalf("Aggregate failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected aggregate result")
	}
}

func TestMustRegisterRejectsDuplicate(t *testing.T) {
	resetRegistryForTest()

	MustRegister(stubHandlerWithType("stub_dup"))

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for duplicate handler")
		}
	}()
	MustRegister(stubHandlerWithType("stub_dup"))
}

func TestMustRegisterImplementationSharesSpec(t *testing.T) {
	resetRegistryForTest()

	spec := stubSpec("realtime_only")
	MustRegisterImplementation(Registration{
		Spec:              spec,
		RealtimeEvaluator: stubRealtimeOnly{},
	})

	resolvedSpec, realtime, err := ResolveRealtime("realtime_only")
	if err != nil {
		t.Fatalf("ResolveRealtime failed: %v", err)
	}
	if resolvedSpec.MessageType() != "realtime_only" {
		t.Fatalf("unexpected message type: %s", resolvedSpec.MessageType())
	}
	if realtime == nil {
		t.Fatal("expected realtime evaluator")
	}

	if _, _, err := ResolveAggregate("realtime_only"); !errors.Is(err, ErrCapabilityNotImplemented) {
		t.Fatalf("expected ErrCapabilityNotImplemented, got %v", err)
	}
}

type stubHandlerWithType string

func (s stubHandlerWithType) MessageType() string { return string(s) }
func (s stubHandlerWithType) NewFilter() any      { return nil }
func (s stubHandlerWithType) Aggregate(_ context.Context, _ *BizAggregateRequest) (*BizAggregateResult, error) {
	return &BizAggregateResult{BizVars: TemplateVars{}}, nil
}
func (s stubHandlerWithType) Evaluate(_ context.Context, _ *RealtimeRequest) (*RealtimeResult, error) {
	return &RealtimeResult{Matched: true, IdempotencyKey: "biz"}, nil
}
