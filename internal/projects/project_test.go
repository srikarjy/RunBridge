package projects_test

import (
	"errors"
	"testing"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/projects"
)

func TestNewProjectRequiresName(t *testing.T) {
	id, err := projects.NewProjectID("project-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = projects.NewProject(id, " ")
	if !errors.Is(err, projects.ErrProjectNameRequired) {
		t.Fatalf("expected ErrProjectNameRequired, got %v", err)
	}
}

func TestMembershipRequiresKnownRole(t *testing.T) {
	projectID, _ := projects.NewProjectID("project-1")
	actorID, _ := auth.NewActorID("actor-1")
	_, err := projects.NewMembership(projectID, actorID, projects.Role("owner"))
	if !errors.Is(err, projects.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}
