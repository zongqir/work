package xdrriskdigest

import (
	"context"

	core "notes/code/aggregate_registry_demo"
	"notes/code/aggregate_registry_demo/aggregators"
	"notes/code/aggregate_registry_demo/messages"
)

type Aggregator struct{}

var _ core.Aggregator = (*Aggregator)(nil)

func init() {
	New().MustRegister()
}

func New() *Aggregator {
	return &Aggregator{}
}

func (a *Aggregator) MustRegister() {
	aggregators.MustRegister(a)
}

func (a *Aggregator) MessageType() string {
	return "xdr_risk_digest"
}

func (a *Aggregator) Aggregate(_ context.Context, req *core.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	_ = req
	return &messages.BizAggregateResult{
		MessageType: a.MessageType(),
		BizVars: messages.TemplateVars{
			// business fills vars here
		},
	}, nil
}
