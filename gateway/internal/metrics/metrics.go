package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Connection metrics
	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_active_connections",
		Help: "Current number of active WebSocket connections",
	})

	SSHSessionsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_ssh_sessions_total",
		Help: "Total number of SSH sessions",
	})

	// Resource metrics
	FDUsagePercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_fd_usage_percent",
		Help: "File descriptor usage percentage",
	})

	// Performance metrics
	BufferPoolHitRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_buffer_pool_hit_rate",
		Help: "Buffer pool hit rate",
	})

	SlowNodesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_slow_nodes_total",
		Help: "Number of slow nodes currently in backpressure",
	})

	StreamRouterLatencyMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "gateway_stream_router_latency_ms",
		Help:    "Stream router tick latency in milliseconds",
		Buckets: []float64{1, 5, 10, 20, 50, 100},
	})

	// Request metrics
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_requests_total",
		Help: "Total number of requests",
	}, []string{"method", "endpoint"})

	RequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_request_duration_seconds",
		Help:    "Request duration in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
	}, []string{"method", "endpoint"})
)

// RecordStreamRouterLatency records the latency of a stream router tick
func RecordStreamRouterLatency(latencyMs float64) {
	StreamRouterLatencyMs.Observe(latencyMs)
}

// Handler returns the Prometheus metrics HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}