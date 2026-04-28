package aggregators

import (
	"fmt"
	"strings"

	core "notes/code/aggregate_registry_demo"
)

var items = map[string]core.Aggregator{}

// MustRegister 由各个实现包在 init 中调用。
// 约束很简单：实现方必须同时满足 Aggregate + MessageType。
func MustRegister(aggregator core.TypedAggregator) {
	if aggregator == nil {
		panic(fmt.Errorf("%w: aggregator is required", core.ErrInvalidRequest))
	}

	messageType := strings.TrimSpace(aggregator.MessageType())
	if messageType == "" {
		panic(fmt.Errorf("%w: message_type is required", core.ErrInvalidRequest))
	}
	if _, exists := items[messageType]; exists {
		panic(fmt.Errorf("%w: duplicate aggregator for %s", core.ErrInvalidRequest, messageType))
	}

	items[messageType] = aggregator
}

func Resolve(messageType string) (core.Aggregator, error) {
	messageType = strings.TrimSpace(messageType)
	if messageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", core.ErrInvalidRequest)
	}

	aggregator, ok := items[messageType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", core.ErrAggregatorNotFound, messageType)
	}
	return aggregator, nil
}
