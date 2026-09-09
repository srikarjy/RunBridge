package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecutionHandlerRequiresConfiguration(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/projects/project-1/executions/execution-1", nil)
	handler := ExecutionHandler{}
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", recorder.Code)
	}
}
