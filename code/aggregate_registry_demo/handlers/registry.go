package handlers

import (
	"fmt"
	"strings"

	core "notes/code/aggregate_registry_demo"
)

var items = map[string]core.Handler{}

func MustRegister(handler core.Handler) {
	if handler == nil {
		panic(fmt.Errorf("%w: handler is required", core.ErrInvalidRequest))
	}

	messageType := strings.TrimSpace(handler.MessageType())
	if messageType == "" {
		panic(fmt.Errorf("%w: message_type is required", core.ErrInvalidRequest))
	}
	if _, exists := items[messageType]; exists {
		panic(fmt.Errorf("%w: duplicate handler for %s", core.ErrInvalidRequest, messageType))
	}

	items[messageType] = handler
}

func Resolve(messageType string) (core.Handler, error) {
	messageType = strings.TrimSpace(messageType)
	if messageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", core.ErrInvalidRequest)
	}

	handler, ok := items[messageType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", core.ErrAggregatorNotFound, messageType)
	}
	return handler, nil
}
