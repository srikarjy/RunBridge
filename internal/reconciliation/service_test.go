package reconciliation

import (
	"context"
	"testing"

	"github.com/srikarjy/RunBridge/internal/observability"
)

type candidateSourceFake struct{ candidates []Candidate }

func (fake candidateSourceFake) ListCandidates(context.Context, int) ([]Candidate, error) {
	return fake.candidates, nil
}

type observerFake struct{ result ObservationResult }

func (fake observerFake) FindSubmission(context.Context, Candidate) (ObservationResult, error) {
	return fake.result, nil
}

type sinkFake struct{ adopted, retried, unknown, manual int }

func (fake *sinkFake) AdoptExecution(context.Context, Candidate, string) error {
	fake.adopted++
	return nil
}
func (fake *sinkFake) AllowRetry(context.Context, Candidate) error  { fake.retried++; return nil }
func (fake *sinkFake) KeepUnknown(context.Context, Candidate) error { fake.unknown++; return nil }
func (fake *sinkFake) RequireManualReview(context.Context, Candidate) error {
	fake.manual++
	return nil
}

func TestServiceAdoptsOnlyCorrelatedObservation(t *testing.T) {
	candidate := Candidate{ExecutionID: "exec-1", WorkspaceID: "ws-1", CorrelationID: "corr-1", AttemptNumber: 1, MaxAttempts: 3}
	sink := &sinkFake{}
	metrics := &observability.Metrics{}
	service := NewService(candidateSourceFake{candidates: []Candidate{candidate}}, observerFake{result: ObservationResult{Observations: []Observation{
		{ExternalID: "wrong", WorkspaceID: "ws-2", CorrelationID: "corr-1"},
		{ExternalID: "right", WorkspaceID: "ws-1", CorrelationID: "corr-1"},
	}}}, sink).WithMetrics(metrics)
	if err := service.ReconcileOnce(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if sink.adopted != 1 || sink.unknown != 0 || metrics.Snapshot().ReconciliationAttempts != 1 {
		t.Fatalf("unexpected decision: %+v metrics=%+v", sink, metrics.Snapshot())
	}
}

func TestServiceKeepsEmptyNonDefinitiveResultUnknown(t *testing.T) {
	candidate := Candidate{ExecutionID: "exec-1", WorkspaceID: "ws-1", CorrelationID: "corr-1", AttemptNumber: 1, MaxAttempts: 3}
	sink := &sinkFake{}
	service := NewService(candidateSourceFake{candidates: []Candidate{candidate}}, observerFake{}, sink)
	if err := service.ReconcileOnce(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if sink.unknown != 1 {
		t.Fatalf("unknown decisions: %d", sink.unknown)
	}
}

func TestServiceAllowsDefinitiveRetryWithinBound(t *testing.T) {
	candidate := Candidate{ExecutionID: "exec-1", WorkspaceID: "ws-1", CorrelationID: "corr-1", AttemptNumber: 1, MaxAttempts: 2}
	sink := &sinkFake{}
	service := NewService(candidateSourceFake{candidates: []Candidate{candidate}}, observerFake{result: ObservationResult{DefinitiveFailure: true}}, sink)
	if err := service.ReconcileOnce(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if sink.retried != 1 {
		t.Fatalf("retry decisions: %d", sink.retried)
	}
}
