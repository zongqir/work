package contract

import (
	"context"
	"errors"
	"testing"
)

type stubHandler struct{}

func resetRegistryForTest() {
	registeredHandlers = map[string]Handler{}
}

func (stubHandler) MessageType() string { return "stub" }
func (stubHandler) NewFilter() any      { return nil }
func (stubHandler) Aggregate(_ context.Context, _ *BizAggregateRequest) (*BizAggregateResult, error) {
	return &BizAggregateResult{BizVars: TemplateVars{}}, nil
}
func (stubHandler) Evaluate(_ context.Context, _ *RealtimeRequest) (*RealtimeResult, error) {
	return &RealtimeResult{Matched: false, IdempotencyKey: ""}, nil
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

type stubHandlerWithType string

func (s stubHandlerWithType) MessageType() string { return string(s) }
func (s stubHandlerWithType) NewFilter() any      { return nil }
func (s stubHandlerWithType) Aggregate(_ context.Context, _ *BizAggregateRequest) (*BizAggregateResult, error) {
	return &BizAggregateResult{BizVars: TemplateVars{}}, nil
}
func (s stubHandlerWithType) Evaluate(_ context.Context, req *RealtimeRequest) (*RealtimeResult, error) {
	return &RealtimeResult{Matched: true, IdempotencyKey: "biz"}, nil
}
