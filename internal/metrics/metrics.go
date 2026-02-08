package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	SSEEventProcessingLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sse_event_processing_latency_seconds",
			Help:    "Latency of SSE event processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	TelegramMessageSendLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "telegram_message_send_latency_seconds",
			Help:    "Latency of Telegram message send operations",
			Buckets: prometheus.DefBuckets,
		},
	)

	ActiveSSEConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_sse_connections",
			Help: "Number of active SSE connections",
		},
	)

	SSEConnectionErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sse_connection_errors_total",
			Help: "Total number of SSE connection errors",
		},
		[]string{"error_type"},
	)
)

func ObserveSSEEventProcessing(eventType string, start time.Time) {
	SSEEventProcessingLatency.WithLabelValues(eventType).Observe(time.Since(start).Seconds())
}

func ObserveTelegramMessageSend(start time.Time) {
	TelegramMessageSendLatency.Observe(time.Since(start).Seconds())
}
