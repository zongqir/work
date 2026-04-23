package notifysdk

import "time"

type Metrics interface {
	RecordSend(mode Mode, bizType string, eventCode string, result string, duration time.Duration)
	RecordError(mode Mode, errorType string)
	RecordDispatch(result string)
	SetOutboxSize(status OutboxStatus, count int)
}

type NoopMetrics struct{}

func (NoopMetrics) RecordSend(Mode, string, string, string, time.Duration) {}

func (NoopMetrics) RecordError(Mode, string) {}

func (NoopMetrics) RecordDispatch(string) {}

func (NoopMetrics) SetOutboxSize(OutboxStatus, int) {}

func classifyError(err error) string {
	switch {
	case err == nil:
		return ""
	case hasError(err, ErrInvalidArgument):
		return "invalid_argument"
	case hasError(err, ErrMarshalPayload):
		return "marshal_payload"
	case hasError(err, ErrTransportTimeout):
		return "transport_timeout"
	case hasError(err, ErrTransportUnavailable):
		return "transport_unavailable"
	case hasError(err, ErrEnqueueFailed):
		return "enqueue_failed"
	case hasError(err, ErrOutboxNotConfigured):
		return "outbox_not_configured"
	default:
		return "unknown"
	}
}
