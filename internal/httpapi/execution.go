package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/srikarjy/RunBridge/internal/authorization"
	"github.com/srikarjy/RunBridge/internal/postgres"
	"github.com/srikarjy/RunBridge/internal/projects"
)

type ExecutionHandler struct {
	Store            *postgres.Store
	Authorizer       *authorization.Authorizer
	ResolvePrincipal PrincipalResolver
}

func (handler *ExecutionHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if handler == nil || handler.Store == nil || handler.Authorizer == nil || handler.ResolvePrincipal == nil {
		http.Error(writer, "execution unavailable", http.StatusServiceUnavailable)
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
	if err := handler.Authorizer.Authorize(request.Context(), principal, projectID, authorization.PermissionRunRead); err != nil {
		if errors.Is(err, authorization.ErrMembershipRequired) || errors.Is(err, authorization.ErrPermissionDenied) {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(writer, "authorization unavailable", http.StatusInternalServerError)
		return
	}
	record, err := handler.Store.GetExecution(request.Context(), projectID.String(), request.PathValue("executionID"))
	if errors.Is(err, postgres.ErrNotFound) {
		http.NotFound(writer, request)
		return
	}
	if err != nil {
		http.Error(writer, "execution unavailable", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(record)
}
