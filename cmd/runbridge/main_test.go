package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"status":"ok"}` {
		t.Fatalf("health response: %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Request-ID") == "" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security/correlation headers: %#v", recorder.Header())
	}
}

func TestConfiguredWebhookRouteIsMounted(t *testing.T) {
	called := false
	webhook := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		called = true
		writer.WriteHeader(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	handlerWithDependencies(nil, nil, webhook).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/webhooks/seqera", strings.NewReader(`{}`)))
	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("webhook route: status=%d called=%v", recorder.Code, called)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "runbridge_submission_failures_total 0") {
		t.Fatalf("metrics response: %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestReadinessRequiresPersistence(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status: %d", recorder.Code)
	}
}
