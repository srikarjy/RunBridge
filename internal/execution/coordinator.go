package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/srikarjy/RunBridge/internal/observability"
	"github.com/srikarjy/RunBridge/internal/runs"
)

var ErrSubmissionUncertain = errors.New("external submission outcome is uncertain")

type AttemptStarter interface {
	CreateAttempt(ctx context.Context, id, executionID, correlationID string, number int64, startedAt time.Time) error
}

type AtomicClaimer interface {
	ClaimSubmission(ctx context.Context, executionID, attemptID, correlationID string, number int64, startedAt time.Time) error
}

type StateWriter interface {
	TransitionExecution(ctx context.Context, id string, expected, next runs.Status, externalWorkspaceID, externalExecutionID *string) error
}

type Launcher interface {
	Submit(ctx context.Context, request LaunchRequest) (LaunchResponse, error)
}

// LaunchRequest and LaunchResponse are intentionally small coordinator
// contracts. An integration adapter can implement Launcher without leaking
// vendor-specific response types into lifecycle code.
type LaunchRequest struct{ WorkspaceID, Pipeline, Revision, ParamsText string }
type LaunchResponse struct{ ExternalExecutionID string }

type Coordinator struct {
	attempts AttemptStarter
	states   StateWriter
	launcher Launcher
	metrics  *observability.Metrics
}

func NewCoordinator(attempts AttemptStarter, states StateWriter, launcher Launcher) *Coordinator {
	return &Coordinator{attempts: attempts, states: states, launcher: launcher}
}

func (coordinator *Coordinator) WithMetrics(metrics *observability.Metrics) *Coordinator {
	coordinator.metrics = metrics
	return coordinator
}

func (coordinator *Coordinator) Submit(ctx context.Context, executionID, attemptID, correlationID string, attemptNumber int64, request LaunchRequest, startedAt time.Time) (runs.Status, string, error) {
	if coordinator == nil || coordinator.attempts == nil || coordinator.states == nil || coordinator.launcher == nil {
		return "", "", errors.New("execution coordinator is not configured")
	}
	if claimer, ok := coordinator.attempts.(AtomicClaimer); ok {
		if err := claimer.ClaimSubmission(ctx, executionID, attemptID, correlationID, attemptNumber, startedAt); err != nil {
			return "", "", fmt.Errorf("claim execution submission: %w", err)
		}
	} else {
		if err := coordinator.attempts.CreateAttempt(ctx, attemptID, executionID, correlationID, attemptNumber, startedAt); err != nil {
			return "", "", fmt.Errorf("create submission attempt: %w", err)
		}
		if err := coordinator.states.TransitionExecution(ctx, executionID, runs.StatusApproved, runs.StatusSubmitting, nil, nil); err != nil {
			if coordinator.metrics != nil {
				coordinator.metrics.TransitionConflicts.Add(1)
			}
			return "", "", fmt.Errorf("claim execution submission: %w", err)
		}
	}
	response, err := coordinator.launcher.Submit(ctx, request)
	if err != nil {
		if coordinator.metrics != nil {
			coordinator.metrics.SubmissionFailures.Add(1)
		}
		_ = coordinator.states.TransitionExecution(ctx, executionID, runs.StatusSubmitting, runs.StatusSubmissionUnknown, nil, nil)
		return runs.StatusSubmissionUnknown, "", fmt.Errorf("%w: %v", ErrSubmissionUncertain, err)
	}
	if response.ExternalExecutionID == "" {
		if coordinator.metrics != nil {
			coordinator.metrics.SubmissionFailures.Add(1)
		}
		_ = coordinator.states.TransitionExecution(ctx, executionID, runs.StatusSubmitting, runs.StatusSubmissionUnknown, nil, nil)
		return runs.StatusSubmissionUnknown, "", ErrSubmissionUncertain
	}
	if err := coordinator.states.TransitionExecution(ctx, executionID, runs.StatusSubmitting, runs.StatusRunning, nil, &response.ExternalExecutionID); err != nil {
		if coordinator.metrics != nil {
			coordinator.metrics.TransitionConflicts.Add(1)
		}
		return "", "", fmt.Errorf("record accepted execution: %w", err)
	}
	return runs.StatusRunning, response.ExternalExecutionID, nil
}

// CancelBeforeSubmission withdraws an approved execution locally. Once
// submission has begun, cancellation must go through the external system and
// a confirmed event before the lifecycle becomes CANCELLED.
func (coordinator *Coordinator) CancelBeforeSubmission(ctx context.Context, executionID string) error {
	if coordinator == nil || coordinator.states == nil {
		return errors.New("execution coordinator is not configured")
	}
	return coordinator.states.TransitionExecution(ctx, executionID, runs.StatusApproved, runs.StatusCancelled, nil, nil)
}
