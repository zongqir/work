package aggregate

import (
	"context"
	"errors"
	"testing"

	"notes/code/aggregate_registry_demo/messages"
)

type stubHandler struct{}

func (stubHandler) MessageType() string { return "stub" }
func (stubHandler) MustRegister()       {}
func (stubHandler) Aggregate(_ context.Context, _ *BizAggregateRequest) (*messages.BizAggregateResult, error) {
	return &messages.BizAggregateResult{MessageType: "stub", BizVars: messages.TemplateVars{}}, nil
}
func (stubHandler) Evaluate(_ context.Context, _ *RealtimeRequest) (*RealtimeDecision, error) {
	return &RealtimeDecision{Matched: false}, nil
}

func TestResolveNotFound(t *testing.T) {
	_, err := Resolve("missing")
	if !errors.Is(err, ErrAggregatorNotFound) {
		t.Fatalf("expected ErrAggregatorNotFound, got %v", err)
	}
}

func TestMustRegisterAndResolve(t *testing.T) {
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
	if result.MessageType != "stub" {
		t.Fatalf("unexpected message type: %s", result.MessageType)
	}
}

type stubHandlerWithType string

func (s stubHandlerWithType) MessageType() string { return string(s) }
func (s stubHandlerWithType) MustRegister()       {}
func (s stubHandlerWithType) Aggregate(_ context.Context, _ *BizAggregateRequest) (*messages.BizAggregateResult, error) {
	return &messages.BizAggregateResult{MessageType: "stub", BizVars: messages.TemplateVars{}}, nil
}
func (s stubHandlerWithType) Evaluate(_ context.Context, _ *RealtimeRequest) (*RealtimeDecision, error) {
	return &RealtimeDecision{Matched: true}, nil
}
