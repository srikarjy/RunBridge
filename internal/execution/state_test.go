package execution

import (
	"errors"
	"testing"

	"github.com/srikarjy/RunBridge/internal/runs"
)

func TestLegalTransitions(t *testing.T) {
	for _, pair := range [][2]runs.Status{
		{runs.StatusApproved, runs.StatusSubmitting},
		{runs.StatusSubmitting, runs.StatusSubmissionUnknown},
		{runs.StatusSubmissionUnknown, runs.StatusRunning},
		{runs.StatusRunning, runs.StatusSucceeded},
	} {
		if err := Transition(pair[0], pair[1]); err != nil {
			t.Fatalf("%s -> %s: %v", pair[0], pair[1], err)
		}
	}
}

func TestIllegalAndTerminalTransitions(t *testing.T) {
	if err := Transition(runs.StatusApproved, runs.StatusRunning); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("expected illegal transition, got %v", err)
	}
	if err := Transition(runs.StatusSucceeded, runs.StatusRunning); !errors.Is(err, ErrTerminalState) {
		t.Fatalf("expected terminal error, got %v", err)
	}
	if err := Transition(runs.StatusRunning, runs.StatusRunning); err != nil {
		t.Fatalf("same state should be idempotent: %v", err)
	}
}

func TestExternalStatusMappingRejectsUnknown(t *testing.T) {
	status, err := NormalizeExternalStatus("queued")
	if err != nil || status != runs.StatusRunning {
		t.Fatalf("queued: %v %v", status, err)
	}
	if _, err := NormalizeExternalStatus("maybe"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expected unknown status error, got %v", err)
	}
}
