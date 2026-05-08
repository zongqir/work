package metrics

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "notification"

var (
	registerOnce    sync.Once
	consumeMessages = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "consume_messages_total",
			Help:      "Total consumed messages by message type and action.",
		},
		[]string{"message_type", "action"},
	)
	consumeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "consume_duration_seconds",
			Help:      "Message consume handling duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"message_type", "action"},
	)
)

func Handler() http.Handler {
	register()
	return promhttp.Handler()
}

func ObserveConsume(messageType, action string, duration time.Duration) {
	register()
	consumeMessages.WithLabelValues(labelValue(messageType), labelValue(action)).Inc()
	consumeDuration.WithLabelValues(labelValue(messageType), labelValue(action)).Observe(duration.Seconds())
}

func register() {
	registerOnce.Do(func() {
		prometheus.MustRegister(
			consumeMessages,
			consumeDuration,
		)
	})
}

func labelValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
