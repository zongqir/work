package xdrriskdigest

import (
	"context"

	core "notes/code/aggregate_registry_demo"
	"notes/code/aggregate_registry_demo/messages"
)

type Handler struct{}

var _ core.Handler = (*Handler)(nil)

func init() {
	New().MustRegister()
}

func New() *Handler {
	return &Handler{}
}

func (h *Handler) MustRegister() {
	core.MustRegister(h)
}

func (h *Handler) MessageType() string {
	return "xdr_risk_digest"
}

func (h *Handler) Aggregate(_ context.Context, req *core.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	_ = req
	return &messages.BizAggregateResult{
		MessageType: h.MessageType(),
		BizVars:     messages.TemplateVars{
			// business fills vars here
		},
	}, nil
}

func (h *Handler) Evaluate(_ context.Context, req *core.RealtimeRequest) (*core.RealtimeDecision, error) {
	_ = req
	return &core.RealtimeDecision{
		Matched: true,
		BizVars: messages.TemplateVars{
			// business fills vars here
		},
	}, nil
}
