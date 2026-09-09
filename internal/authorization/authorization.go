// Package authorization evaluates project-scoped permissions for authenticated
// principals. It does not authenticate credentials or implement an HTTP layer.
package authorization

import (
	"context"
	"errors"
	"fmt"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/projects"
)

var (
	ErrMembershipRequired = errors.New("project membership is required")
	ErrPermissionDenied   = errors.New("permission denied")
)

type Permission string

const (
	PermissionProjectRead   Permission = "project.read"
	PermissionProjectManage Permission = "project.manage"
	PermissionRunRead       Permission = "run.read"
	PermissionRunPropose    Permission = "run.propose"
	PermissionRunReview     Permission = "run.review"
	PermissionRunCancel     Permission = "run.cancel"
	PermissionAuditRead     Permission = "audit.read"
)

// Allows is the single role-to-permission mapping used by the domain boundary.
// It is intentionally code-defined until policy requirements justify a richer
// configuration model.
func Allows(role projects.Role, permission Permission) bool {
	switch role {
	case projects.RoleAdmin:
		return true
	case projects.RoleReviewer:
		return permission == PermissionProjectRead || permission == PermissionRunRead || permission == PermissionRunReview || permission == PermissionAuditRead
	case projects.RoleRunner:
		return permission == PermissionProjectRead || permission == PermissionRunRead || permission == PermissionRunPropose || permission == PermissionRunCancel || permission == PermissionAuditRead
	case projects.RoleViewer:
		return permission == PermissionProjectRead || permission == PermissionRunRead || permission == PermissionAuditRead
	default:
		return false
	}
}

type MembershipReader interface {
	MembershipFor(ctx context.Context, projectID projects.ProjectID, actorID auth.ActorID) (projects.Membership, error)
}

type Authorizer struct {
	memberships MembershipReader
}

func NewAuthorizer(memberships MembershipReader) *Authorizer {
	return &Authorizer{memberships: memberships}
}

// Authorize resolves membership for the requested project on every call. The
// caller cannot supply a role or use membership from another project.
func (authorizer *Authorizer) Authorize(ctx context.Context, principal auth.Actor, projectID projects.ProjectID, permission Permission) error {
	if authorizer == nil || authorizer.memberships == nil {
		return errors.New("authorization membership reader is not configured")
	}
	membership, err := authorizer.memberships.MembershipFor(ctx, projectID, principal.ID())
	if errors.Is(err, ErrMembershipRequired) || errors.Is(err, projects.ErrMembershipNotFound) {
		return ErrMembershipRequired
	}
	if err != nil {
		return fmt.Errorf("resolve project membership: %w", err)
	}
	if membership.ProjectID() != projectID || membership.ActorID() != principal.ID() {
		return ErrMembershipRequired
	}
	if !Allows(membership.Role(), permission) {
		return fmt.Errorf("%s: %w", permission, ErrPermissionDenied)
	}
	return nil
}
