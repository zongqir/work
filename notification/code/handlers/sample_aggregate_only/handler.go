package sampleaggregateonly

import (
	"context"

	"work/notification/code/contract"
)

type Handler struct{}

type Filter struct {
	Severity []string `json:"severity"`
}

func init() {
	handler := &Handler{}
	contract.MustRegisterImplementation(contract.Registration{
		Spec:      handler,
		Aggregate: handler,
	})
}

func (h *Handler) MessageType() string {
	return "sample_aggregate_only"
}

func (h *Handler) NewFilter() any {
	return &Filter{}
}

func (h *Handler) Aggregate(_ context.Context, req *contract.BizAggregateRequest) (*contract.BizAggregateResult, error) {
	filter, _ := req.Filter.(*Filter)

	severityCount := 0
	if filter != nil {
		severityCount = len(filter.Severity)
	}

	return &contract.BizAggregateResult{
		BizVars: contract.TemplateVars{
			"severity_count": severityCount,
		},
	}, nil
}
