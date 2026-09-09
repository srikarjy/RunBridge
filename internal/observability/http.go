package observability

import (
	"log/slog"
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

// Middleware creates a server-owned request correlation ID, returns it to the
// caller, and emits one structured completion record with method, path, and
// status. A client cannot overwrite the ID used for log correlation.
func Middleware(next http.Handler, logger *slog.Logger) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID, err := NewID()
		if err != nil {
			http.Error(writer, "request correlation unavailable", http.StatusInternalServerError)
			return
		}
		ctx := WithRequestID(request.Context(), requestID)
		request = request.WithContext(ctx)
		writer.Header().Set("X-Request-ID", requestID)
		started := time.Now()
		captured := &statusWriter{ResponseWriter: writer}
		next.ServeHTTP(captured, request)
		status := captured.status
		if status == 0 {
			status = http.StatusOK
		}
		Logger(logger, request.Context()).Info("http request completed", "method", request.Method, "path", request.URL.Path, "status", status, "duration_ms", time.Since(started).Milliseconds())
	})
}
