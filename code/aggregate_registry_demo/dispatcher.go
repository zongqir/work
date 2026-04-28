package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MessagePublisher 由平台提供，负责把命中的实时结果发布出去。
// 默认口径可以是发到 MQ，不承诺具体介质。
type MessagePublisher interface {
	Publish(ctx context.Context, msg *DispatchMessage) error
}

type Dispatcher struct {
	Publisher MessagePublisher
	LoadAll   func(ctx context.Context) (map[string]map[string]json.RawMessage, error)

	cache configCache
}

type messageConfig struct {
	Enabled         bool            `json:"enabled"`
	AggregateFilter json.RawMessage `json:"aggregate_filter"`
	RealtimeFilter  json.RawMessage `json:"realtime_filter"`
}

func (d *Dispatcher) SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) error {
	handler, config, tenantID, err := d.prepare(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if config == nil || !config.Enabled {
		return nil
	}

	return d.sendAggregateWithHandler(ctx, handler, &BizAggregateRequest{
		TenantID:    tenantID,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		ConfigBody:  config.AggregateFilter,
	})
}

func (d *Dispatcher) SendRealtime(ctx context.Context, tenantID, messageType string, eventBody json.RawMessage) error {
	handler, config, tenantID, err := d.prepare(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if config == nil || !config.Enabled {
		return nil
	}

	_, err = d.sendRealtimeWithHandler(ctx, handler, &RealtimeRequest{
		TenantID:    tenantID,
		FilterQuery: config.RealtimeFilter,
		EventBody:   eventBody,
	})
	return err
}

func (d *Dispatcher) prepare(ctx context.Context, tenantID, messageType string) (Handler, *messageConfig, string, error) {
	if d == nil || d.Publisher == nil {
		return nil, nil, "", fmt.Errorf("%w: message publisher is required", ErrInvalidRequest)
	}
	if d.LoadAll == nil {
		return nil, nil, "", fmt.Errorf("%w: load_all is required", ErrInvalidRequest)
	}

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, nil, "", fmt.Errorf("%w: tenant_id is required", ErrInvalidRequest)
	}
	messageType = strings.TrimSpace(messageType)
	if messageType == "" {
		return nil, nil, "", fmt.Errorf("%w: message_type is required", ErrInvalidRequest)
	}

	handler, err := Resolve(messageType)
	if err != nil {
		return nil, nil, "", err
	}

	configBody, err := d.cache.pick(ctx, tenantID, messageType, d.LoadAll)
	if err != nil {
		return nil, nil, "", err
	}
	if len(configBody) == 0 {
		return handler, nil, tenantID, nil
	}

	var config messageConfig
	if err := json.Unmarshal(configBody, &config); err != nil {
		return nil, nil, "", err
	}
	return handler, &config, tenantID, nil
}

func (d *Dispatcher) dispatch(ctx context.Context, msg *DispatchMessage) error {
	if d == nil || d.Publisher == nil {
		return fmt.Errorf("%w: message publisher is required", ErrInvalidRequest)
	}
	if msg == nil {
		return fmt.Errorf("%w: dispatch message is nil", ErrInvalidRequest)
	}

	return d.Publisher.Publish(ctx, msg)
}

func (d *Dispatcher) sendAggregateWithHandler(ctx context.Context, handler Handler, req *BizAggregateRequest) error {
	if handler == nil {
		return fmt.Errorf("%w: handler is required", ErrInvalidRequest)
	}
	if req == nil {
		return fmt.Errorf("%w: aggregate request is nil", ErrInvalidRequest)
	}

	result, err := handler.Aggregate(ctx, req)
	if err != nil {
		return err
	}
	if result == nil || len(result.BizVars) == 0 {
		return nil
	}

	messageType := result.MessageType
	if strings.TrimSpace(messageType) == "" {
		messageType = handler.MessageType()
	}

	return d.dispatch(ctx, &DispatchMessage{
		TenantID:    req.TenantID,
		MessageType: messageType,
		BizVars:     result.BizVars,
	})
}

func (d *Dispatcher) sendRealtimeWithHandler(ctx context.Context, handler Handler, req *RealtimeRequest) (*RealtimeDecision, error) {
	if handler == nil {
		return nil, fmt.Errorf("%w: handler is required", ErrInvalidRequest)
	}
	if req == nil {
		return nil, fmt.Errorf("%w: realtime request is nil", ErrInvalidRequest)
	}

	decision, err := handler.Evaluate(ctx, req)
	if err != nil {
		return nil, err
	}
	if decision == nil {
		return nil, fmt.Errorf("%w: realtime decision is nil", ErrTemporaryFailure)
	}
	if !decision.Matched {
		return decision, nil
	}

	if err := d.dispatch(ctx, &DispatchMessage{
		TenantID:    req.TenantID,
		MessageType: handler.MessageType(),
		BizVars:     decision.BizVars,
		EventBody:   req.EventBody,
	}); err != nil {
		return nil, err
	}

	return decision, nil
}
