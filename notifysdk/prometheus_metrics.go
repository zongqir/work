package notifysdk

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	errorsTotal     *prometheus.CounterVec
	dispatchTotal   *prometheus.CounterVec
	outboxSize      *prometheus.GaugeVec
}

func NewPrometheusMetrics(reg prometheus.Registerer, namespace string) *PrometheusMetrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	m := &PrometheusMetrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "notify_sdk",
				Name:      "requests_total",
				Help:      "Total notification SDK send requests.",
			},
			[]string{"mode", "biz_type", "event_code", "result"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "notify_sdk",
				Name:      "request_duration_seconds",
				Help:      "Notification SDK send duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"mode", "biz_type", "event_code"},
		),
		errorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "notify_sdk",
				Name:      "errors_total",
				Help:      "Total notification SDK errors by mode and type.",
			},
			[]string{"mode", "error_type"},
		),
		dispatchTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "notify_sdk",
				Name:      "dispatch_total",
				Help:      "Outbox dispatcher results.",
			},
			[]string{"result"},
		),
		outboxSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "notify_sdk",
				Name:      "outbox_size",
				Help:      "Current outbox size by status.",
			},
			[]string{"status"},
		),
	}

	reg.MustRegister(
		m.requestsTotal,
		m.requestDuration,
		m.errorsTotal,
		m.dispatchTotal,
		m.outboxSize,
	)

	return m
}

func (m *PrometheusMetrics) RecordSend(mode Mode, bizType string, eventCode string, result string, duration time.Duration) {
	m.requestsTotal.WithLabelValues(string(mode), bizType, eventCode, result).Inc()
	m.requestDuration.WithLabelValues(string(mode), bizType, eventCode).Observe(duration.Seconds())
}

func (m *PrometheusMetrics) RecordError(mode Mode, errorType string) {
	m.errorsTotal.WithLabelValues(string(mode), errorType).Inc()
}

func (m *PrometheusMetrics) RecordDispatch(result string) {
	m.dispatchTotal.WithLabelValues(result).Inc()
}

func (m *PrometheusMetrics) SetOutboxSize(status OutboxStatus, count int) {
	m.outboxSize.WithLabelValues(string(status)).Set(float64(count))
}
