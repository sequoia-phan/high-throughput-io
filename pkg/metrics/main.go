package main

import (
	"io/pkg/telemetry"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	mux := http.NewServeMux()

	// Business Handlers (Ví dụ: Log Ingestion & Read Query)
	mux.HandleFunc("POST /api/v1/logs", handleIngestLogs)
	mux.HandleFunc("GET /api/v1/logs", handleQueryLogs)

	// Expose Prometheus Metrics Endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Bọc middleware đo đạc xung quanh Mux
	handler := telemetry.MetricsMiddleware(mux)

	log.Println("Go Orchestrator started on :8082 with /metrics exposed")
	if err := http.ListenAndServe(":8082", handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleIngestLogs(w http.ResponseWriter, r *http.Request) {
	// Giả lập cập nhật IPC Queue Depth
	telemetry.IPCQueueDepth.Inc()
	defer telemetry.IPCQueueDepth.Dec()

	// Logic đẩy payload sang Rust Engine ...
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func handleQueryLogs(w http.ResponseWriter, r *http.Request) {
	// Logic Read-Path ...
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"data":[]}`))
}
