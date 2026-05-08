package sampleaggregateonly

import (
	"context"

	"work/notification/code/pkg/notification"
)

type Handler struct{}

type Filter struct {
	Severity []string `json:"severity"`
}

func init() {
	handler := &Handler{}
	notification.MustRegisterImplementation(notification.Registration{
		Spec:              handler,
		AggregateProvider: handler,
	})
}

func (h *Handler) MessageType() string {
	return "sample_aggregate_only"
}

func (h *Handler) NewFilter() any {
	return &Filter{}
}

func (h *Handler) Aggregate(_ context.Context, req *notification.BizAggregateRequest) (*notification.BizAggregateResult, error) {
	filter, _ := req.Filter.(*Filter)

	severityCount := 0
	if filter != nil {
		severityCount = len(filter.Severity)
	}

	return &notification.BizAggregateResult{
		BizVars: notification.TemplateVars{
			"severity_count": severityCount,
		},
	}, nil
}
