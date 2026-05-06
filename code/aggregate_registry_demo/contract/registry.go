package contract

import (
	"fmt"
	"sync"
)

var registryMu sync.RWMutex
var registeredHandlers = map[string]Handler{}
var registryFrozen bool

func MustRegister(handler Handler) {
	if handler == nil {
		panic(fmt.Errorf("%w: handler is required", ErrInvalidRequest))
	}

	messageType := handler.MessageType()
	if messageType == "" {
		panic(fmt.Errorf("%w: message_type is required", ErrInvalidRequest))
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if registryFrozen {
		panic(fmt.Errorf("%w: registry is frozen", ErrInvalidRequest))
	}
	if _, exists := registeredHandlers[messageType]; exists {
		panic(fmt.Errorf("%w: duplicate handler for %s", ErrInvalidRequest, messageType))
	}

	registeredHandlers[messageType] = handler
}

func Resolve(messageType string) (Handler, error) {
	if messageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", ErrInvalidRequest)
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	registryFrozen = true
	handler, ok := registeredHandlers[messageType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAggregatorNotFound, messageType)
	}
	return handler, nil
}
