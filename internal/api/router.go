package api

import (
	"io/internal/config"
	"io/internal/storage"
	"io/pkg/telemetry"
	"net/http"
)

func NewRouter(cfg *config.Config, engine storage.EngineBridge) http.Handler {
	mux := http.NewServeMux()
	logHandler := NewLogHandler(engine)

	mux.HandleFunc("GET /ready", readyHandler)

	// API v1
	mux.HandleFunc("GET /api/v1/status", statusHandler(cfg))

	// Ingest API for clients
	mux.HandleFunc("POST /api/v1/logs", logHandler.IngestLogs)

	// Probes
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"alive"}`))
	})

	return LoggerMiddleware(telemetry.MetricsMiddleware(mux))
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"alive"}`))
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

func statusHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"env":"` + cfg.Env + `","service":"orchestrator"}`))
	}
}
