package xdrriskdigest

import (
	"context"

	"notes/code/aggregate_registry_demo/contract"
	"notes/code/aggregate_registry_demo/messages"
)

type Handler struct{}

var _ contract.Handler = (*Handler)(nil)

func init() {
	New().MustRegister()
}

func New() *Handler {
	return &Handler{}
}

func (h *Handler) MustRegister() {
	contract.MustRegister(h)
}

func (h *Handler) MessageType() string {
	return "xdr_risk_digest"
}

func (h *Handler) Aggregate(_ context.Context, req *contract.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	_ = req
	return &messages.BizAggregateResult{
		BizVars: messages.TemplateVars{
			// business fills vars here
		},
	}, nil
}

func (h *Handler) Evaluate(_ context.Context, req *contract.RealtimeRequest) (*contract.RealtimeDecision, error) {
	_ = req
	return &contract.RealtimeDecision{
		Matched: true,
		BizVars: messages.TemplateVars{
			// business fills vars here
		},
	}, nil
}
