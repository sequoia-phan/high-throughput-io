package api

import (
	"encoding/json"
	"io/internal/storage"
	"net/http"
)

type LogHandler struct {
	rustEngine storage.EngineBridge
}

func NewLogHandler(engine storage.EngineBridge) *LogHandler {
	return &LogHandler{rustEngine: engine}
}

func (h *LogHandler) IngestLogs(w http.ResponseWriter, r *http.Request) {
	var entries []storage.LogEntry

	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		http.Error(w, `{"error":"invalid_payload"}`, http.StatusBadRequest)
		return
	}

	if len(entries) == 0 {
		http.Error(w, `{"error":"empty_batch"}`, http.StatusBadRequest)
		return
	}

	if err := h.rustEngine.WriteBatch(r.Context(), entries); err != nil {
		http.Error(w, `{"error":"storage_engine_failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`))

}
