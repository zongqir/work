package xdrriskdigest

import (
	"context"

	"notes/code/aggregate_registry_demo/contract"
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
	contract.MustRegister(&Handler{})
}

func (h *Handler) MessageType() string {
	return "xdr_risk_digest"
}

func (h *Handler) NewFilter() any {
	return &Filter{}
}

func (h *Handler) Aggregate(_ context.Context, req *contract.BizAggregateRequest) (*contract.BizAggregateResult, error) {
	filter, _ := req.Filter.(*Filter)
	_ = filter

	return &contract.BizAggregateResult{
		BizVars: contract.TemplateVars{
			// business fills vars here
		},
	}, nil
}

func (h *Handler) Evaluate(_ context.Context, req *contract.RealtimeRequest) (*contract.RealtimeResult, error) {
	filter, _ := req.Filter.(*Filter)
	_ = filter

	event, _ := req.Event.(*RealtimeEvent)

	idempotencyKey := ""
	if event != nil {
		idempotencyKey = event.EventID
	}

	return &contract.RealtimeResult{
		Matched:        true,
		IdempotencyKey: idempotencyKey,
		BizVars:        contract.TemplateVars{
			// business fills vars here
		},
	}, nil
}
