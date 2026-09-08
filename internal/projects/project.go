// Package projects defines project ownership and membership concepts.
package projects

import (
	"errors"
	"strings"

	"github.com/srikarjy/RunBridge/internal/auth"
)

var (
	ErrProjectIDRequired   = errors.New("project ID is required")
	ErrProjectNameRequired = errors.New("project name is required")
	ErrInvalidRole         = errors.New("project role is invalid")
)

type ProjectID string

func NewProjectID(value string) (ProjectID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrProjectIDRequired
	}
	return ProjectID(value), nil
}

func (id ProjectID) String() string { return string(id) }

type Project struct {
	id   ProjectID
	name string
}

func NewProject(id ProjectID, name string) (Project, error) {
	if strings.TrimSpace(id.String()) == "" {
		return Project{}, ErrProjectIDRequired
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, ErrProjectNameRequired
	}
	return Project{id: id, name: name}, nil
}

func (project Project) ID() ProjectID { return project.id }
func (project Project) Name() string  { return project.name }

// Role is a label only. Permission grants are defined in Phase 3.
type Role string

const (
	RoleViewer   Role = "viewer"
	RoleRunner   Role = "runner"
	RoleReviewer Role = "reviewer"
	RoleAdmin    Role = "admin"
)

func (role Role) Valid() bool {
	switch role {
	case RoleViewer, RoleRunner, RoleReviewer, RoleAdmin:
		return true
	default:
		return false
	}
}

type Membership struct {
	projectID ProjectID
	actorID   auth.ActorID
	role      Role
}

func NewMembership(projectID ProjectID, actorID auth.ActorID, role Role) (Membership, error) {
	if strings.TrimSpace(projectID.String()) == "" {
		return Membership{}, ErrProjectIDRequired
	}
	if strings.TrimSpace(actorID.String()) == "" {
		return Membership{}, auth.ErrActorIDRequired
	}
	if !role.Valid() {
		return Membership{}, ErrInvalidRole
	}
	return Membership{projectID: projectID, actorID: actorID, role: role}, nil
}

func (membership Membership) ProjectID() ProjectID  { return membership.projectID }
func (membership Membership) ActorID() auth.ActorID { return membership.actorID }
func (membership Membership) Role() Role            { return membership.role }
