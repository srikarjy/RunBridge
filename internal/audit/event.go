// Package audit defines immutable, append-oriented audit records.
package audit

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type ActorKind string

const (
	Human    ActorKind = "human"
	System   ActorKind = "system"
	External ActorKind = "external"
)

const (
	ProposalCreated       = "proposal.created"
	PreflightCompleted    = "preflight.completed"
	DiffCalculated        = "diff.calculated"
	ApprovalRequested     = "approval.requested"
	ApprovalDecided       = "approval.decided"
	ExecutionSubmitted    = "execution.submitted"
	ExecutionStateChanged = "execution.state_changed"
	ExecutionCompleted    = "execution.completed"
	ArtifactRecorded      = "artifact.recorded"
)

var ErrInvalidEvent = errors.New("invalid audit event")

type Event struct {
	ID               string
	ProjectID        string
	ProposalID       string
	ActorID          string
	ActorKind        ActorKind
	Type             string
	ObjectType       string
	ObjectID         string
	CorrelationID    string
	SourceSystem     string
	SourceEventID    string
	SourceOccurredAt *time.Time
	RecordedAt       time.Time
	Metadata         map[string]any
}

func (event Event) Validate() error {
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.ProjectID) == "" || strings.TrimSpace(event.Type) == "" || event.RecordedAt.IsZero() {
		return fmt.Errorf("%w: identity and recorded timestamp are required", ErrInvalidEvent)
	}
	switch event.ActorKind {
	case Human:
		if strings.TrimSpace(event.ActorID) == "" {
			return fmt.Errorf("%w: human events require actor", ErrInvalidEvent)
		}
	case System, External:
		if strings.TrimSpace(event.ActorID) != "" {
			return fmt.Errorf("%w: system/external events cannot have actor", ErrInvalidEvent)
		}
	default:
		return fmt.Errorf("%w: unsupported actor kind %q", ErrInvalidEvent, event.ActorKind)
	}
	if (strings.TrimSpace(event.SourceSystem) == "") != (strings.TrimSpace(event.SourceEventID) == "") {
		return fmt.Errorf("%w: source system and event ID must be paired", ErrInvalidEvent)
	}
	return nil
}

func (event Event) CloneMetadata() map[string]any {
	if event.Metadata == nil {
		return map[string]any{}
	}
	clone := make(map[string]any, len(event.Metadata))
	for key, value := range event.Metadata {
		clone[key] = value
	}
	return clone
}
