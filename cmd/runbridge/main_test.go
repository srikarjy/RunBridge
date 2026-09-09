package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"status":"ok"}` {
		t.Fatalf("health response: %d %q", recorder.Code, recorder.Body.String())
	}
}
