package aggregate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"notes/code/aggregate_registry_demo/messages"
)

var (
	ErrInvalidRequest     = errors.New("invalid aggregate request")
	ErrUnsupportedConfig  = errors.New("unsupported aggregate config")
	ErrTemporaryFailure   = errors.New("temporary aggregate failure")
	ErrAggregatorNotFound = errors.New("aggregator not found")
)

var registeredHandlers = map[string]Handler{}

// BizAggregateRequest 是发给业务方聚合接口的请求。
// 这里只保留平台自己需要的上下文，例如 tenant_id 和查询条件。
type BizAggregateRequest struct {
	TenantID    string          `json:"tenant_id"`
	WindowStart time.Time       `json:"window_start"`
	WindowEnd   time.Time       `json:"window_end"`
	ConfigBody  json.RawMessage `json:"config_body"`
}

type RealtimeRequest struct {
	TenantID    string          `json:"tenant_id"`
	FilterQuery json.RawMessage `json:"filter_query"`
	EventBody   json.RawMessage `json:"event_body"`
}

type RealtimeDecision struct {
	Matched bool                  `json:"matched"`
	BizVars messages.TemplateVars `json:"biz_vars,omitempty"`
}

type DispatchMessage struct {
	TenantID    string                `json:"tenant_id"`
	MessageType string                `json:"message_type"`
	BizVars     messages.TemplateVars `json:"biz_vars"`
	EventBody   json.RawMessage       `json:"event_body,omitempty"`
}

// Handler 是业务侧需要实现的最小生产契约。
// 一个 handler 同时声明聚合和实时两种能力，少实现任何一个方法都无法通过编译。
type Handler interface {
	MessageType() string
	MustRegister()
	Aggregate(ctx context.Context, req *BizAggregateRequest) (*messages.BizAggregateResult, error)
	Evaluate(ctx context.Context, req *RealtimeRequest) (*RealtimeDecision, error)
}

func MustRegister(handler Handler) {
	if handler == nil {
		panic(fmt.Errorf("%w: handler is required", ErrInvalidRequest))
	}

	messageType := strings.TrimSpace(handler.MessageType())
	if messageType == "" {
		panic(fmt.Errorf("%w: message_type is required", ErrInvalidRequest))
	}
	if _, exists := registeredHandlers[messageType]; exists {
		panic(fmt.Errorf("%w: duplicate handler for %s", ErrInvalidRequest, messageType))
	}

	registeredHandlers[messageType] = handler
}

func Resolve(messageType string) (Handler, error) {
	messageType = strings.TrimSpace(messageType)
	if messageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", ErrInvalidRequest)
	}

	handler, ok := registeredHandlers[messageType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAggregatorNotFound, messageType)
	}
	return handler, nil
}
