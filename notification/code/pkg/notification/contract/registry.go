package contract

import (
	"context"
	"fmt"
	"sort"
)

var registeredImplementations = map[string]Registration{}

func MustRegister(spec MessageTypeSpec) {
	if spec == nil {
		panic(fmt.Errorf("%w: message type spec is required", ErrInvalidRequest))
	}

	registration := Registration{
		Spec: spec,
	}
	if realtime, ok := spec.(RealtimeEvaluator); ok {
		registration.RealtimeEvaluator = realtime
	}
	if aggregate, ok := spec.(AggregateProvider); ok {
		registration.AggregateProvider = aggregate
	}

	register(registration)
}

func MustRegisterImplementation(registration Registration) {
	register(registration)
}

func Resolve(messageType string) (Handler, error) {
	registration, err := resolveRegistration(messageType)
	if err != nil {
		return nil, err
	}
	if registration.RealtimeEvaluator == nil || registration.AggregateProvider == nil {
		return nil, fmt.Errorf("%w: full handler is not implemented for %s", ErrCapabilityNotImplemented, messageType)
	}

	return registration, nil
}

func ResolveSpec(messageType string) (MessageTypeSpec, error) {
	registration, err := resolveRegistration(messageType)
	if err != nil {
		return nil, err
	}
	return registration.Spec, nil
}

func ResolveRealtime(messageType string) (MessageTypeSpec, RealtimeEvaluator, error) {
	registration, err := resolveRegistration(messageType)
	if err != nil {
		return nil, nil, err
	}
	if registration.RealtimeEvaluator == nil {
		return nil, nil, fmt.Errorf("%w: realtime evaluator is not implemented for %s", ErrCapabilityNotImplemented, messageType)
	}
	return registration.Spec, registration.RealtimeEvaluator, nil
}

func ResolveAggregate(messageType string) (MessageTypeSpec, AggregateProvider, error) {
	registration, err := resolveRegistration(messageType)
	if err != nil {
		return nil, nil, err
	}
	if registration.AggregateProvider == nil {
		return nil, nil, fmt.Errorf("%w: aggregate provider is not implemented for %s", ErrCapabilityNotImplemented, messageType)
	}
	return registration.Spec, registration.AggregateProvider, nil
}

func RegisteredMessageTypes() []string {
	items := make([]string, 0, len(registeredImplementations))
	for messageType := range registeredImplementations {
		items = append(items, messageType)
	}
	sort.Strings(items)
	return items
}

func register(registration Registration) {
	if registration.Spec == nil {
		panic(fmt.Errorf("%w: message type spec is required", ErrInvalidRequest))
	}
	if registration.RealtimeEvaluator == nil && registration.AggregateProvider == nil {
		panic(fmt.Errorf("%w: at least one capability is required", ErrInvalidRequest))
	}

	messageType := registration.Spec.MessageType()
	if messageType == "" {
		panic(fmt.Errorf("%w: message_type is required", ErrInvalidRequest))
	}
	if _, exists := registeredImplementations[messageType]; exists {
		panic(fmt.Errorf("%w: duplicate handler for %s", ErrInvalidRequest, messageType))
	}

	registeredImplementations[messageType] = registration
}

func resolveRegistration(messageType string) (Registration, error) {
	if messageType == "" {
		return Registration{}, fmt.Errorf("%w: message_type is required", ErrInvalidRequest)
	}

	registration, ok := registeredImplementations[messageType]
	if !ok {
		return Registration{}, fmt.Errorf("%w: %s", ErrHandlerNotFound, messageType)
	}
	return registration, nil
}

func (r Registration) MessageType() string {
	return r.Spec.MessageType()
}

func (r Registration) NewFilter() any {
	return r.Spec.NewFilter()
}

func (r Registration) Evaluate(ctx context.Context, req *RealtimeRequest) (*RealtimeResult, error) {
	return r.RealtimeEvaluator.Evaluate(ctx, req)
}

func (r Registration) Aggregate(ctx context.Context, req *BizAggregateRequest) (*BizAggregateResult, error) {
	return r.AggregateProvider.Aggregate(ctx, req)
}
