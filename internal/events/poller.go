package events

import (
	"context"
	"fmt"
	"time"

	"github.com/srikarjy/RunBridge/internal/runs"
)

type PollCandidate struct {
	ExternalExecutionID string
	CurrentStatus       runs.Status
}

type PollCandidateSource interface {
	ListRunningExecutions(context.Context, int) ([]PollCandidate, error)
}

type StatusObserver interface {
	WorkflowStatus(context.Context, string) (runs.Status, error)
}

type EventProcessor interface {
	Process(context.Context, Event) (Decision, error)
}

type Poller struct {
	source   PollCandidateSource
	observer StatusObserver
	process  EventProcessor
	interval time.Duration
	limit    int
	onError  func(error)
}

func NewPoller(source PollCandidateSource, observer StatusObserver, process EventProcessor, interval time.Duration, limit int) (*Poller, error) {
	if source == nil || observer == nil || process == nil || interval <= 0 {
		return nil, fmt.Errorf("execution poller is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return &Poller{source: source, observer: observer, process: process, interval: interval, limit: limit}, nil
}

func (poller *Poller) WithErrorHandler(handler func(error)) *Poller {
	poller.onError = handler
	return poller
}

func (poller *Poller) PollOnce(ctx context.Context) error {
	candidates, err := poller.source.ListRunningExecutions(ctx, poller.limit)
	if err != nil {
		return fmt.Errorf("list running executions: %w", err)
	}
	for _, candidate := range candidates {
		status, err := poller.observer.WorkflowStatus(ctx, candidate.ExternalExecutionID)
		if err != nil {
			return fmt.Errorf("observe workflow %s: %w", candidate.ExternalExecutionID, err)
		}
		if status == candidate.CurrentStatus {
			continue
		}
		event := Event{
			Source:              "seqera-poll",
			ID:                  fmt.Sprintf("%s:%s", candidate.ExternalExecutionID, status),
			ExternalExecutionID: candidate.ExternalExecutionID,
			Status:              status,
			OccurredAt:          time.Now().UTC(),
		}
		if _, err := poller.process.Process(ctx, event); err != nil {
			return fmt.Errorf("process workflow %s: %w", candidate.ExternalExecutionID, err)
		}
	}
	return nil
}

func (poller *Poller) Run(ctx context.Context) error {
	poller.poll(ctx)
	ticker := time.NewTicker(poller.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			poller.poll(ctx)
		}
	}
}

func (poller *Poller) poll(ctx context.Context) {
	if err := poller.PollOnce(ctx); err != nil && poller.onError != nil {
		poller.onError(err)
	}
}
