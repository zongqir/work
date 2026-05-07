package samplerealtimeonly

import (
	"context"

	"work/notification/code/contract"
)

type Handler struct{}

type Filter struct {
	Severity []string `json:"severity"`
}

type Event struct {
	EventID  string `json:"event_id"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
}

func init() {
	handler := &Handler{}
	contract.MustRegisterImplementation(contract.Registration{
		Spec:     handler,
		Realtime: handler,
	})
}

func (h *Handler) MessageType() string {
	return "sample_realtime_only"
}

func (h *Handler) NewFilter() any {
	return &Filter{}
}

func (h *Handler) Evaluate(_ context.Context, req *contract.RealtimeRequest) (*contract.RealtimeResult, error) {
	filter, _ := req.Filter.(*Filter)
	event, _ := req.Event.(*Event)

	if event == nil {
		return &contract.RealtimeResult{Matched: false}, nil
	}
	if !contains(filter, event.Severity) {
		return &contract.RealtimeResult{Matched: false}, nil
	}

	return &contract.RealtimeResult{
		Matched:        true,
		IdempotencyKey: event.EventID,
		BizVars: contract.TemplateVars{
			"title":    event.Title,
			"severity": event.Severity,
		},
	}, nil
}

func contains(filter *Filter, severity string) bool {
	if filter == nil || len(filter.Severity) == 0 {
		return true
	}
	for _, item := range filter.Severity {
		if item == severity {
			return true
		}
	}
	return false
}
