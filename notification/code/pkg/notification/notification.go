package notification

import (
	"context"

	"work/notification/code/pkg/notification/contract"
)

var (
	ErrInvalidRequest           = contract.ErrInvalidRequest
	ErrUnsupportedConfig        = contract.ErrUnsupportedConfig
	ErrTemporaryFailure         = contract.ErrTemporaryFailure
	ErrHandlerNotFound          = contract.ErrHandlerNotFound
	ErrCapabilityNotImplemented = contract.ErrCapabilityNotImplemented
)

type DelayError = contract.DelayError
type DispatchMessage = contract.DispatchMessage
type TemplateVars = contract.TemplateVars
type BizAggregateRequest = contract.BizAggregateRequest
type BizAggregateResult = contract.BizAggregateResult
type RealtimeRequest = contract.RealtimeRequest
type RealtimeResult = contract.RealtimeResult

const (
	DispatchSourceAggregate = contract.DispatchSourceAggregate
	DispatchSourceRealtime  = contract.DispatchSourceRealtime
)

type MessageTypeSpec = contract.MessageTypeSpec
type RealtimeEvaluator = contract.RealtimeEvaluator
type AggregateProvider = contract.AggregateProvider
type RealtimeHandler = contract.RealtimeHandler
type AggregateHandler = contract.AggregateHandler
type Handler = contract.Handler
type Registration = contract.Registration

func MustRegister(spec MessageTypeSpec) {
	contract.MustRegister(spec)
}

func MustRegisterImplementation(registration Registration) {
	contract.MustRegisterImplementation(registration)
}

func Resolve(messageType string) (Handler, error) {
	return contract.Resolve(messageType)
}

func ResolveSpec(messageType string) (MessageTypeSpec, error) {
	return contract.ResolveSpec(messageType)
}

func ResolveRealtime(messageType string) (MessageTypeSpec, RealtimeEvaluator, error) {
	return contract.ResolveRealtime(messageType)
}

func ResolveAggregate(messageType string) (MessageTypeSpec, AggregateProvider, error) {
	return contract.ResolveAggregate(messageType)
}

func RegisteredMessageTypes() []string {
	return contract.RegisteredMessageTypes()
}

func EvaluateRealtime(ctx context.Context, messageType string, req *RealtimeRequest) (*RealtimeResult, error) {
	_, handler, err := ResolveRealtime(messageType)
	if err != nil {
		return nil, err
	}
	return handler.Evaluate(ctx, req)
}
