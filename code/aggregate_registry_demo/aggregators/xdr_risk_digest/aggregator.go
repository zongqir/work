package xdrriskdigest

import (
	"context"

	core "notes/code/aggregate_registry_demo"
	"notes/code/aggregate_registry_demo/aggregators"
	"notes/code/aggregate_registry_demo/messages"
)

const MessageType = "xdr_risk_digest"

type Aggregator struct{}

var _ core.TypedAggregator = (*Aggregator)(nil)

func init() {
	aggregators.MustRegister(&Aggregator{})
}

func (a *Aggregator) MessageType() string {
	return MessageType
}

func (a *Aggregator) Aggregate(_ context.Context, _ *core.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	return &messages.BizAggregateResult{
		MessageType: MessageType,
		BizVars: messages.TemplateVars{
			// business fills vars here
		},
	}, nil
}
