package events

import (
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/runs"
)

func event(at time.Time, status runs.Status) Event {
	return Event{Source: "seqera", ID: "evt-1", ExternalExecutionID: "wf-1", Status: status, OccurredAt: at}
}

func TestEventDecisions(t *testing.T) {
	now := time.Now().UTC()
	if got, _ := Decide(runs.StatusRunning, now.Add(-time.Minute), event(now, runs.StatusSucceeded)); got != Apply {
		t.Fatalf("new event: %s", got)
	}
	if got, _ := Decide(runs.StatusRunning, now, event(now, runs.StatusRunning)); got != Duplicate {
		t.Fatalf("duplicate: %s", got)
	}
	if got, _ := Decide(runs.StatusRunning, now, event(now.Add(-time.Second), runs.StatusFailed)); got != Stale {
		t.Fatalf("stale: %s", got)
	}
	if got, _ := Decide(runs.StatusSucceeded, now, event(now.Add(time.Second), runs.StatusFailed)); got != Conflict {
		t.Fatalf("terminal conflict: %s", got)
	}
}

func TestEventValidation(t *testing.T) {
	if err := (Event{}).Validate(); err == nil {
		t.Fatal("expected validation error")
	}
	if event(time.Now(), runs.StatusRunning).DeduplicationKey() != "seqera:evt-1" {
		t.Fatal("unexpected deduplication key")
	}
}
