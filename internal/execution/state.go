// Package execution defines the durable execution state vocabulary and legal
// transitions. Persistence and external coordination are deliberately kept
// outside this package.
package execution

import (
	"errors"
	"fmt"
	"strings"

	"github.com/srikarjy/RunBridge/internal/runs"
)

var (
	ErrInvalidState      = errors.New("invalid execution state")
	ErrIllegalTransition = errors.New("illegal execution state transition")
	ErrTerminalState     = errors.New("execution is terminal")
)

type AttemptStatus string

const (
	AttemptPending  AttemptStatus = "pending"
	AttemptAccepted AttemptStatus = "accepted"
	AttemptUnknown  AttemptStatus = "unknown"
	AttemptFailed   AttemptStatus = "failed"
)

func (status AttemptStatus) Valid() bool {
	switch status {
	case AttemptPending, AttemptAccepted, AttemptUnknown, AttemptFailed:
		return true
	default:
		return false
	}
}

type Source string

const (
	SourceCoordinator Source = "coordinator"
	SourceSeqera      Source = "seqera"
	SourceReconciler  Source = "reconciler"
)

var allowed = map[runs.Status]map[runs.Status]struct{}{
	runs.StatusApproved:          {runs.StatusSubmitting: {}},
	runs.StatusSubmitting:        {runs.StatusRunning: {}, runs.StatusSubmissionUnknown: {}, runs.StatusFailed: {}, runs.StatusCancelled: {}},
	runs.StatusSubmissionUnknown: {runs.StatusRunning: {}, runs.StatusSubmitting: {}, runs.StatusFailed: {}, runs.StatusCancelled: {}},
	runs.StatusRunning:           {runs.StatusSucceeded: {}, runs.StatusFailed: {}, runs.StatusCancelled: {}},
	runs.StatusSucceeded:         {},
	runs.StatusFailed:            {},
	runs.StatusCancelled:         {},
}

func ValidateState(status runs.Status) error {
	if _, ok := allowed[status]; !ok {
		return fmt.Errorf("%w: %q", ErrInvalidState, status)
	}
	return nil
}

func CanTransition(from, to runs.Status) bool {
	if ValidateState(from) != nil || ValidateState(to) != nil {
		return false
	}
	if from == to {
		return true
	} // idempotent observations are safe.
	_, ok := allowed[from][to]
	return ok
}

// Transition validates a conditional state change. The caller must persist
// the returned status and audit event atomically with an expected-version
// predicate; this package never hides a concurrent update.
func Transition(from, to runs.Status) error {
	if err := ValidateState(from); err != nil {
		return err
	}
	if err := ValidateState(to); err != nil {
		return err
	}
	if from == to {
		return nil
	}
	if _, ok := allowed[from][to]; !ok {
		if len(allowed[from]) == 0 {
			return fmt.Errorf("%w: %s cannot leave %s", ErrTerminalState, from, to)
		}
		return fmt.Errorf("%w: %s -> %s", ErrIllegalTransition, from, to)
	}
	return nil
}

// NormalizeExternalStatus maps the stable Seqera status values used by the
// coordinator. Unknown values remain explicit errors instead of being guessed.
func NormalizeExternalStatus(value string) (runs.Status, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SUBMITTED", "QUEUED", "STARTING", "RUNNING":
		return runs.StatusRunning, nil
	case "SUCCEEDED", "COMPLETED":
		return runs.StatusSucceeded, nil
	case "FAILED", "ERROR":
		return runs.StatusFailed, nil
	case "CANCELLED", "CANCELED":
		return runs.StatusCancelled, nil
	default:
		return "", fmt.Errorf("%w: unknown Seqera status %q", ErrInvalidState, value)
	}
}
