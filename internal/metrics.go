package internal

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// total requests processed
	MetricsRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lb_requests_total",
			Help: "Total number of HTTP requests processed, separated by backend and status.",
		},
		[]string{"backend_id", "status_code"},
	)

	// current active connections (saturation)
	MetricsActiveConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lb_active_connections",
			Help: "Current number of requests per backend.",
		},
		[]string{"backend_id"},
	)

	// request duration in seconds
	MetricsRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lb_request_duration_seconds",
			Help:    "Histogram of response latency (seconds) per backend.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"backend_id"},
	)
)
