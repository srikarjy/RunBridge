// Package httpapi contains transport handlers assembled by the future service
// entry point. Authentication is injected; handlers never infer identity.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/authorization"
	"github.com/srikarjy/RunBridge/internal/postgres"
	"github.com/srikarjy/RunBridge/internal/projects"
)

type PrincipalResolver func(*http.Request) (auth.Actor, error)

type AuditHandler struct {
	Store            *postgres.Store
	Authorizer       *authorization.Authorizer
	ResolvePrincipal PrincipalResolver
}

func (handler *AuditHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if handler == nil || handler.Store == nil || handler.Authorizer == nil || handler.ResolvePrincipal == nil {
		http.Error(writer, "audit unavailable", http.StatusServiceUnavailable)
		return
	}
	projectID, err := projects.NewProjectID(strings.TrimSpace(request.PathValue("projectID")))
	if err != nil {
		http.Error(writer, "invalid project", http.StatusBadRequest)
		return
	}
	principal, err := handler.ResolvePrincipal(request)
	if err != nil {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := handler.Authorizer.Authorize(request.Context(), principal, projectID, authorization.PermissionAuditRead); err != nil {
		if errors.Is(err, authorization.ErrMembershipRequired) || errors.Is(err, authorization.ErrPermissionDenied) {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(writer, "authorization unavailable", http.StatusInternalServerError)
		return
	}
	limit := 100
	after := int64(0)
	if raw := strings.TrimSpace(request.URL.Query().Get("after")); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed < 0 {
			http.Error(writer, "invalid audit cursor", http.StatusBadRequest)
			return
		}
		after = parsed
	}
	records, err := handler.Store.ListAuditEventsAfter(request.Context(), projectID.String(), after, limit)
	if err != nil {
		http.Error(writer, "audit unavailable", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(records)
}
