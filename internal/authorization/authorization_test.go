package authorization_test

import (
	"context"
	"errors"
	"testing"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/authorization"
	"github.com/srikarjy/RunBridge/internal/projects"
)

type membershipReader struct {
	membership projects.Membership
	err        error
}

func (reader membershipReader) MembershipFor(context.Context, projects.ProjectID, auth.ActorID) (projects.Membership, error) {
	return reader.membership, reader.err
}

func TestRolePermissions(t *testing.T) {
	cases := []struct {
		role       projects.Role
		permission authorization.Permission
		allowed    bool
	}{
		{projects.RoleViewer, authorization.PermissionRunRead, true},
		{projects.RoleViewer, authorization.PermissionRunPropose, false},
		{projects.RoleRunner, authorization.PermissionRunPropose, true},
		{projects.RoleRunner, authorization.PermissionRunReview, false},
		{projects.RoleReviewer, authorization.PermissionRunReview, true},
		{projects.RoleAdmin, authorization.PermissionProjectManage, true},
	}
	for _, testCase := range cases {
		if got := authorization.Allows(testCase.role, testCase.permission); got != testCase.allowed {
			t.Errorf("Allows(%q, %q) = %v, want %v", testCase.role, testCase.permission, got, testCase.allowed)
		}
	}
}

func TestAuthorizeUsesProjectScopedMembership(t *testing.T) {
	actorID, _ := auth.NewActorID("actor-1")
	actor, _ := auth.NewActor(actorID, "Researcher", auth.ActorKindHuman)
	projectID, _ := projects.NewProjectID("project-1")
	membership, _ := projects.NewMembership(projectID, actorID, projects.RoleRunner)
	authorizer := authorization.NewAuthorizer(membershipReader{membership: membership})
	if err := authorizer.Authorize(context.Background(), actor, projectID, authorization.PermissionRunPropose); err != nil {
		t.Fatalf("authorize runner: %v", err)
	}
	otherProjectID, _ := projects.NewProjectID("project-2")
	if err := authorizer.Authorize(context.Background(), actor, otherProjectID, authorization.PermissionRunPropose); !errors.Is(err, authorization.ErrMembershipRequired) {
		t.Fatalf("cross-project authorization error = %v, want ErrMembershipRequired", err)
	}
}

func TestAuthorizeRejectsPermissionAndMissingMembership(t *testing.T) {
	actorID, _ := auth.NewActorID("actor-1")
	actor, _ := auth.NewActor(actorID, "Researcher", auth.ActorKindHuman)
	projectID, _ := projects.NewProjectID("project-1")
	membership, _ := projects.NewMembership(projectID, actorID, projects.RoleViewer)
	authorizer := authorization.NewAuthorizer(membershipReader{membership: membership})
	if err := authorizer.Authorize(context.Background(), actor, projectID, authorization.PermissionRunPropose); !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("viewer authorization error = %v, want ErrPermissionDenied", err)
	}
	authorizer = authorization.NewAuthorizer(membershipReader{err: projects.ErrMembershipNotFound})
	if err := authorizer.Authorize(context.Background(), actor, projectID, authorization.PermissionRunRead); !errors.Is(err, authorization.ErrMembershipRequired) {
		t.Fatalf("missing membership error = %v, want ErrMembershipRequired", err)
	}
}
