package xdrriskdigest

import (
	"context"

	"notes/code/aggregate_registry_demo/contract"
	"notes/code/aggregate_registry_demo/messages"
)

type Handler struct{}

type Filter struct {
	Severity    []string `json:"severity"`
	SampleLimit int      `json:"sample_limit"`
}

type RealtimeEvent struct {
	EventID string `json:"event_id"`
}

var _ contract.Handler = (*Handler)(nil)

func init() {
	contract.MustRegister(New())
}

func New() *Handler {
	return &Handler{}
}

func (h *Handler) MessageType() string {
	return "xdr_risk_digest"
}

func (h *Handler) NewFilter() any {
	return &Filter{}
}

func (h *Handler) NewRealtimeEvent() any {
	return &RealtimeEvent{}
}

func (h *Handler) Aggregate(_ context.Context, req *contract.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	filter, _ := req.Filter.(*Filter)
	_ = filter
	return &messages.BizAggregateResult{
		BizVars: messages.TemplateVars{
			// business fills vars here
		},
	}, nil
}

func (h *Handler) Evaluate(_ context.Context, req *contract.RealtimeRequest) (*contract.RealtimeDecision, error) {
	filter, _ := req.Filter.(*Filter)
	event, _ := req.Event.(*RealtimeEvent)
	_ = filter
	_ = event
	return &contract.RealtimeDecision{
		Matched: true,
		BizVars: messages.TemplateVars{
			// business fills vars here
		},
	}, nil
}

func (h *Handler) RealtimeIdempotencyKey(_ context.Context, req *contract.RealtimeRequest) (string, error) {
	_ = req
	return "fill-business-key-here", nil
}
