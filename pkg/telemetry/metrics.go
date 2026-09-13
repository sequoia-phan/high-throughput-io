package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (

	// 1. HTTP request counter
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed by Go Gateway",
		},
		[]string{"method", "endpoint", "status"},
	)

	// 2. HTTP Latency Histogram
	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"method", "endpoint"},
	)

	// 3. Queue Depth
	IPCQueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ipc_queue_depth",
			Help: "Current number of payloads waiting in the buffer queue for Rust Engine",
		},
	)

	// 4. Time spent writing one log entry to the Unix domain socket.
	UDSWriteDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "uds_write_duration_seconds",
			Help:    "Time spent writing one log entry to the Rust Unix domain socket",
			Buckets: []float64{.00005, .0001, .0005, .001, .005, .01, .05, .1},
		},
	)

	// Most recent successful or failed UDS write duration. Unlike the histogram,
	// this gauge is directly queryable as a single PromQL series.
	UDSWriteLatencySeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "uds_write_latency_seconds",
		Help: "Duration of the most recent write attempt to the Rust Unix domain socket",
	})

	UDSConnectionAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "uds_connection_attempts_total",
			Help: "Total attempts to connect to the Rust Unix domain socket",
		},
		[]string{"result"},
	)

	UDSWriteFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uds_write_failures_total",
		Help: "Total failed writes to the Rust Unix domain socket",
	})

	// Counter for logs dropped before they can enter the Go UDS queue.
	DroppedLogsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "logs_dropped_total",
		Help: "Total number of logs dropped due to queue overflow",
	})
)

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		statusStr := strconv.Itoa(rw.statusCode)

		HttpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
		HttpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
