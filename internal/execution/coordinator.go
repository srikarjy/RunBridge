package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/srikarjy/RunBridge/internal/runs"
)

var ErrSubmissionUncertain = errors.New("external submission outcome is uncertain")

type AttemptStarter interface {
	CreateAttempt(ctx context.Context, id, executionID, correlationID string, number int64, startedAt time.Time) error
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
}

func NewCoordinator(attempts AttemptStarter, states StateWriter, launcher Launcher) *Coordinator {
	return &Coordinator{attempts: attempts, states: states, launcher: launcher}
}

func (coordinator *Coordinator) Submit(ctx context.Context, executionID, attemptID, correlationID string, attemptNumber int64, request LaunchRequest, startedAt time.Time) (runs.Status, string, error) {
	if coordinator == nil || coordinator.attempts == nil || coordinator.states == nil || coordinator.launcher == nil {
		return "", "", errors.New("execution coordinator is not configured")
	}
	if err := coordinator.attempts.CreateAttempt(ctx, attemptID, executionID, correlationID, attemptNumber, startedAt); err != nil {
		return "", "", fmt.Errorf("create submission attempt: %w", err)
	}
	if err := coordinator.states.TransitionExecution(ctx, executionID, runs.StatusApproved, runs.StatusSubmitting, nil, nil); err != nil {
		return "", "", fmt.Errorf("claim execution submission: %w", err)
	}
	response, err := coordinator.launcher.Submit(ctx, request)
	if err != nil {
		_ = coordinator.states.TransitionExecution(ctx, executionID, runs.StatusSubmitting, runs.StatusSubmissionUnknown, nil, nil)
		return runs.StatusSubmissionUnknown, "", fmt.Errorf("%w: %v", ErrSubmissionUncertain, err)
	}
	if response.ExternalExecutionID == "" {
		_ = coordinator.states.TransitionExecution(ctx, executionID, runs.StatusSubmitting, runs.StatusSubmissionUnknown, nil, nil)
		return runs.StatusSubmissionUnknown, "", ErrSubmissionUncertain
	}
	if err := coordinator.states.TransitionExecution(ctx, executionID, runs.StatusSubmitting, runs.StatusRunning, nil, &response.ExternalExecutionID); err != nil {
		return "", "", fmt.Errorf("record accepted execution: %w", err)
	}
	return runs.StatusRunning, response.ExternalExecutionID, nil
}
