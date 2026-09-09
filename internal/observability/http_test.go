package observability

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareAddsServerOwnedRequestID(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := Middleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if RequestID(request.Context()) == "" {
			t.Fatal("request ID missing from context")
		}
		writer.WriteHeader(http.StatusAccepted)
	}), logger)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("X-Request-ID", "client-value")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request.WithContext(context.Background()))
	if recorder.Code != http.StatusAccepted || recorder.Header().Get("X-Request-ID") == "" || recorder.Header().Get("X-Request-ID") == "client-value" {
		t.Fatalf("response correlation: status=%d id=%q", recorder.Code, recorder.Header().Get("X-Request-ID"))
	}
	if !bytes.Contains(logs.Bytes(), []byte("http request completed")) || !bytes.Contains(logs.Bytes(), []byte("request_id")) {
		t.Fatalf("completion log missing: %s", logs.String())
	}
}

func TestMiddlewareRecordsHTTPMetrics(t *testing.T) {
	metrics := &Metrics{}
	handler := MiddlewareWithMetrics(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
	}), slog.Default(), metrics)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/upstream", nil))
	snapshot := metrics.Snapshot()
	if snapshot.HTTPRequests != 1 || snapshot.HTTPErrors != 1 {
		t.Fatalf("metrics: %+v", snapshot)
	}
}
