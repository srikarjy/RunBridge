// Package events defines external execution events before they are persisted
// or delivered through an HTTP webhook.
package events

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/srikarjy/RunBridge/internal/runs"
)

var (
	ErrInvalidEvent          = errors.New("invalid execution event")
	ErrEventIdentityRequired = errors.New("event source and ID are required")
)

type Event struct {
	Source              string
	ID                  string
	ExternalExecutionID string
	Status              runs.Status
	OccurredAt          time.Time
}

func (event Event) Validate() error {
	if strings.TrimSpace(event.Source) == "" || strings.TrimSpace(event.ID) == "" {
		return fmt.Errorf("%w: %w", ErrInvalidEvent, ErrEventIdentityRequired)
	}
	if strings.TrimSpace(event.ExternalExecutionID) == "" || event.OccurredAt.IsZero() {
		return fmt.Errorf("%w: execution ID and occurrence time are required", ErrInvalidEvent)
	}
	if event.Status != runs.StatusRunning && event.Status != runs.StatusSucceeded && event.Status != runs.StatusFailed && event.Status != runs.StatusCancelled {
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidEvent, event.Status)
	}
	return nil
}

func (event Event) DeduplicationKey() string {
	return strings.TrimSpace(event.Source) + ":" + strings.TrimSpace(event.ID)
}

type Decision string

const (
	Apply     Decision = "apply"
	Duplicate Decision = "duplicate"
	Stale     Decision = "stale"
	Conflict  Decision = "conflict"
)

// Decide determines whether an event can affect local state. Equal timestamps
// are accepted only when they repeat the same status; contradictory terminal
// evidence is surfaced for reconciliation instead of silently overwritten.
func Decide(current runs.Status, currentOccurredAt time.Time, event Event) (Decision, error) {
	if err := event.Validate(); err != nil {
		return Conflict, err
	}
	if !currentOccurredAt.IsZero() && event.OccurredAt.Before(currentOccurredAt) {
		return Stale, nil
	}
	if event.OccurredAt.Equal(currentOccurredAt) && current == event.Status {
		return Duplicate, nil
	}
	if current == runs.StatusSucceeded || current == runs.StatusFailed || current == runs.StatusCancelled {
		if current == event.Status {
			return Duplicate, nil
		}
		return Conflict, nil
	}
	return Apply, nil
}
