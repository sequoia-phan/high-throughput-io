package api

import (
	"net/http"
)

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// start := time.Now()
		wrapped := &respomseWriterWrapped{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		// slog.Info("http_request",
		// 	"method", r.Method,
		// 	"path", r.URL.Path,
		// 	"status", wrapped.statusCode,
		// 	"latency_ms", time.Since(start).Microseconds(), "remote_ip", r.RemoteAddr)
	})
}

type respomseWriterWrapped struct {
	http.ResponseWriter
	statusCode int
}

func (rw *respomseWriterWrapped) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
