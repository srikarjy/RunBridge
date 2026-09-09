package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/authorization"
	"github.com/srikarjy/RunBridge/internal/postgres"
)

func TestAuditHandlerRequiresConfiguration(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/projects/project-1/audit", nil)
	handler := AuditHandler{}
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", recorder.Code)
	}
}

func TestAuditHandlerDoesNotAcceptMissingPrincipal(t *testing.T) {
	resolver := func(*http.Request) (auth.Actor, error) { return auth.Actor{}, errors.New("missing") }
	handler := AuditHandler{Store: &postgres.Store{}, Authorizer: authorization.NewAuthorizer(nil), ResolvePrincipal: resolver}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/projects/project-1/audit", nil)
	request.SetPathValue("projectID", "project-1")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", recorder.Code)
	}
}
