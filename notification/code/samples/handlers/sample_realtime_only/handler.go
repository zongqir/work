package samplerealtimeonly

import (
	"context"

	"work/notification/code/pkg/notification"
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
	notification.MustRegisterImplementation(notification.Registration{
		Spec:              handler,
		RealtimeEvaluator: handler,
	})
}

func (h *Handler) MessageType() string {
	return "sample_realtime_only"
}

func (h *Handler) NewFilter() any {
	return &Filter{}
}

func (h *Handler) Evaluate(_ context.Context, req *notification.RealtimeRequest) (*notification.RealtimeResult, error) {
	filter, _ := req.Filter.(*Filter)
	event, _ := req.Event.(*Event)

	if event == nil {
		return &notification.RealtimeResult{Matched: false}, nil
	}
	if !contains(filter, event.Severity) {
		return &notification.RealtimeResult{Matched: false}, nil
	}

	return &notification.RealtimeResult{
		Matched:        true,
		IdempotencyKey: event.EventID,
		BizVars: notification.TemplateVars{
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
