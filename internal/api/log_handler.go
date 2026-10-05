package api

import (
	"encoding/json"
	"errors"
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
		if errors.Is(err, storage.ErrBufferFull) {
			http.Error(w, `{"error":"storage_queue_full"}`, http.StatusServiceUnavailable)
			return
		}
		if r.Context().Err() != nil {
			http.Error(w, `{"error":"request_cancelled"}`, http.StatusRequestTimeout)
			return
		}
		http.Error(w, `{"error":"storage_engine_failed"}`, http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`))

}
