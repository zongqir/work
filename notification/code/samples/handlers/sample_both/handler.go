package sampleboth

import (
	"context"

	"work/notification/code/pkg/notification"
)

type Handler struct{}

type Filter struct {
	Severity    []string `json:"severity"`
	SampleLimit int      `json:"sample_limit"`
}

type RealtimeEvent struct {
	EventID string `json:"event_id"`
}

func init() {
	notification.MustRegister(&Handler{})
}

func (h *Handler) MessageType() string {
	return "sample_both"
}

func (h *Handler) NewFilter() any {
	return &Filter{}
}

func (h *Handler) Aggregate(_ context.Context, req *notification.BizAggregateRequest) (*notification.BizAggregateResult, error) {
	filter, _ := req.Filter.(*Filter)
	_ = filter

	return &notification.BizAggregateResult{
		BizVars: notification.TemplateVars{
			// business fills vars here
		},
	}, nil
}

func (h *Handler) Evaluate(_ context.Context, req *notification.RealtimeRequest) (*notification.RealtimeResult, error) {
	filter, _ := req.Filter.(*Filter)
	_ = filter

	event, _ := req.Event.(*RealtimeEvent)

	idempotencyKey := ""
	if event != nil {
		idempotencyKey = event.EventID
	}

	return &notification.RealtimeResult{
		Matched:        true,
		IdempotencyKey: idempotencyKey,
		BizVars:        notification.TemplateVars{
			// business fills vars here
		},
	}, nil
}
