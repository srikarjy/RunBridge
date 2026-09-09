package events

import (
	"context"
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/observability"
	"github.com/srikarjy/RunBridge/internal/runs"
)

type lookupFake struct{ view ExecutionView }

func (fake lookupFake) LookupExternalExecution(context.Context, string) (ExecutionView, error) {
	return fake.view, nil
}

type applierFake struct{ decision Decision }

func (fake applierFake) ApplyExternalEvent(context.Context, Event, string, string, runs.Status, time.Time, time.Time) (Decision, error) {
	return fake.decision, nil
}

func TestProcessorCountsDuplicateDelivery(t *testing.T) {
	metrics := &observability.Metrics{}
	processor := NewProcessor(lookupFake{view: ExecutionView{ID: "exec-1", ProjectID: "project-1", Status: runs.StatusRunning}}, applierFake{decision: Duplicate}).WithMetrics(metrics)
	event := Event{Source: "seqera", ID: "evt-1", ExternalExecutionID: "external-1", Status: runs.StatusSucceeded, OccurredAt: time.Now()}
	decision, err := processor.Process(context.Background(), event)
	if err != nil || decision != Duplicate {
		t.Fatalf("decision=%q err=%v", decision, err)
	}
	if metrics.Snapshot().WebhookDuplicates != 1 {
		t.Fatalf("duplicate metric=%d", metrics.Snapshot().WebhookDuplicates)
	}
}

func TestProcessorRejectsIncompleteLookup(t *testing.T) {
	processor := NewProcessor(lookupFake{}, applierFake{decision: Apply})
	event := Event{Source: "seqera", ID: "evt-1", ExternalExecutionID: "external-1", Status: runs.StatusSucceeded, OccurredAt: time.Now()}
	if _, err := processor.Process(context.Background(), event); err == nil {
		t.Fatal("expected incomplete lookup error")
	}
}
