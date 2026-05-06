package contract

import (
	"context"
	"errors"
	"testing"

	"notes/code/aggregate_registry_demo/messages"
)

type stubHandler struct{}

func resetRegistryForTest() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registeredHandlers = map[string]Handler{}
	registryFrozen = false
}

func (stubHandler) MessageType() string { return "stub" }
func (stubHandler) NewFilter() any      { return nil }
func (stubHandler) Aggregate(_ context.Context, _ *BizAggregateRequest) (*messages.BizAggregateResult, error) {
	return &messages.BizAggregateResult{BizVars: messages.TemplateVars{}}, nil
}
func (stubHandler) Evaluate(_ context.Context, _ *RealtimeRequest) (*RealtimeDecision, error) {
	return &RealtimeDecision{Matched: false, IdempotencyKey: ""}, nil
}

func TestResolveNotFound(t *testing.T) {
	resetRegistryForTest()

	_, err := Resolve("missing")
	if !errors.Is(err, ErrAggregatorNotFound) {
		t.Fatalf("expected ErrAggregatorNotFound, got %v", err)
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

func TestRegistryFrozenAfterResolve(t *testing.T) {
	resetRegistryForTest()

	MustRegister(stubHandlerWithType("stub_before_freeze"))
	if _, err := Resolve("stub_before_freeze"); err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic after registry is frozen")
		}
	}()
	MustRegister(stubHandlerWithType("stub_after_freeze"))
}

type stubHandlerWithType string

func (s stubHandlerWithType) MessageType() string { return string(s) }
func (s stubHandlerWithType) NewFilter() any      { return nil }
func (s stubHandlerWithType) Aggregate(_ context.Context, _ *BizAggregateRequest) (*messages.BizAggregateResult, error) {
	return &messages.BizAggregateResult{BizVars: messages.TemplateVars{}}, nil
}
func (s stubHandlerWithType) Evaluate(_ context.Context, req *RealtimeRequest) (*RealtimeDecision, error) {
	return &RealtimeDecision{Matched: true, IdempotencyKey: "biz"}, nil
}
