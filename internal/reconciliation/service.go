package reconciliation

import (
	"context"
	"fmt"

	"github.com/srikarjy/RunBridge/internal/observability"
	"github.com/srikarjy/RunBridge/internal/runs"
)

// Candidate is a locally durable submission that needs an external lookup.
// CorrelationID is generated per attempt and must be supplied to the external
// integration when the backend supports correlation metadata.
type Candidate struct {
	ExecutionID    string
	WorkspaceID    string
	CorrelationID  string
	AttemptNumber  int64
	MaxAttempts    int64
	ExpectedStatus runs.Status
}

type CandidateSource interface {
	ListCandidates(context.Context, int) ([]Candidate, error)
}

type ObservationResult struct {
	Observations      []Observation
	DefinitiveFailure bool
}

type Observer interface {
	FindSubmission(context.Context, Candidate) (ObservationResult, error)
}

// DecisionSink applies a decision transactionally. Implementations should
// condition updates on the candidate's current durable state so a concurrent
// webhook or worker cannot overwrite newer evidence.
type DecisionSink interface {
	AdoptExecution(context.Context, Candidate, string) error
	AllowRetry(context.Context, Candidate) error
	KeepUnknown(context.Context, Candidate) error
	RequireManualReview(context.Context, Candidate) error
}

type Service struct {
	source   CandidateSource
	observer Observer
	sink     DecisionSink
	metrics  *observability.Metrics
}

func NewService(source CandidateSource, observer Observer, sink DecisionSink) *Service {
	return &Service{source: source, observer: observer, sink: sink}
}

func (service *Service) WithMetrics(metrics *observability.Metrics) *Service {
	service.metrics = metrics
	return service
}

// ReconcileOnce performs one bounded pass. Empty external results remain
// unknown unless the integration explicitly reports a definitive failure;
// this avoids duplicate expensive launches during eventual consistency.
func (service *Service) ReconcileOnce(ctx context.Context, limit int) error {
	if service == nil || service.source == nil || service.observer == nil || service.sink == nil {
		return fmt.Errorf("reconciliation service is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	candidates, err := service.source.ListCandidates(ctx, limit)
	if err != nil {
		return fmt.Errorf("list reconciliation candidates: %w", err)
	}
	for _, candidate := range candidates {
		if service.metrics != nil {
			service.metrics.ReconciliationAttempts.Add(1)
		}
		result, err := service.observer.FindSubmission(ctx, candidate)
		if err != nil {
			return fmt.Errorf("observe execution %s: %w", candidate.ExecutionID, err)
		}
		matches := FindMatches(result.Observations, candidate.WorkspaceID, candidate.CorrelationID)
		decision := Decide(matches, result.DefinitiveFailure)
		var applyErr error
		switch decision.Kind {
		case AdoptExecution:
			applyErr = service.sink.AdoptExecution(ctx, candidate, decision.ExternalID)
		case RetryAllowed:
			if err := CanRetry(candidate.AttemptNumber, candidate.MaxAttempts, true); err != nil {
				applyErr = service.sink.KeepUnknown(ctx, candidate)
			} else {
				applyErr = service.sink.AllowRetry(ctx, candidate)
			}
		case RemainUnknown:
			applyErr = service.sink.KeepUnknown(ctx, candidate)
		case ManualReview:
			applyErr = service.sink.RequireManualReview(ctx, candidate)
		default:
			applyErr = fmt.Errorf("unknown reconciliation decision %q", decision.Kind)
		}
		if applyErr != nil {
			return fmt.Errorf("apply reconciliation decision for %s: %w", candidate.ExecutionID, applyErr)
		}
	}
	return nil
}
