package contract

import (
	"fmt"
)

var registeredHandlers = map[string]Handler{}

func MustRegister(handler Handler) {
	if handler == nil {
		panic(fmt.Errorf("%w: handler is required", ErrInvalidRequest))
	}

	messageType := handler.MessageType()
	if messageType == "" {
		panic(fmt.Errorf("%w: message_type is required", ErrInvalidRequest))
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

	handler, ok := registeredHandlers[messageType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrHandlerNotFound, messageType)
	}
	return handler, nil
}
