package aggregate

import (
	"context"
	"fmt"
)

// PendingWriter 由 SDK 调用，负责把命中的实时结果写入待发送事实表。
type PendingWriter interface {
	WritePending(ctx context.Context, msg *PendingMessage) error
}

type RealtimeSDK struct {
	Writer PendingWriter
}

func (s *RealtimeSDK) Handle(ctx context.Context, handler Handler, req *RealtimeRequest) (*RealtimeDecision, error) {
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
	if s == nil || s.Writer == nil {
		return nil, fmt.Errorf("%w: pending writer is required", ErrInvalidRequest)
	}

	if err := s.Writer.WritePending(ctx, &PendingMessage{
		TenantID:    req.TenantID,
		MessageType: handler.MessageType(),
		BizVars:     decision.BizVars,
		EventBody:   req.EventBody,
	}); err != nil {
		return nil, err
	}

	return decision, nil
}
