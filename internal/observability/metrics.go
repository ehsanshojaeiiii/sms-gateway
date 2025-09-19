package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	HTTPRequestsTotal      *prometheus.CounterVec
	HTTPRequestDuration    *prometheus.HistogramVec
	MessagesProcessedTotal *prometheus.CounterVec
	ActiveConnections      prometheus.Gauge
	CreditOperationsTotal  *prometheus.CounterVec
	QueueDepth             prometheus.Gauge
	RetryAttemptsTotal     *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code", "client_id"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status_code"},
		),
		MessagesProcessedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "messages_processed_total",
				Help: "Total number of messages processed",
			},
			[]string{"status", "provider"},
		),
		ActiveConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_connections",
				Help: "Number of active connections",
			},
		),
		CreditOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "credit_operations_total",
				Help: "Total number of credit operations",
			},
			[]string{"operation", "client_id"},
		),
		QueueDepth: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "queue_depth",
				Help: "Current queue depth",
			},
		),
		RetryAttemptsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "retry_attempts_total",
				Help: "Total number of retry attempts",
			},
			[]string{"reason"},
		),
	}
}
