package events

import (
	"context"
	"fmt"
	"time"

	"github.com/srikarjy/RunBridge/internal/observability"
	"github.com/srikarjy/RunBridge/internal/runs"
)

type ExecutionView struct {
	ID          string
	ProjectID   string
	Status      runs.Status
	LastEventAt time.Time
}

type ExecutionLookup interface {
	LookupExternalExecution(context.Context, string) (ExecutionView, error)
}

type EventApplier interface {
	ApplyExternalEvent(context.Context, Event, string, string, runs.Status, time.Time, time.Time) (Decision, error)
}

// Processor connects authenticated transport events to the durable store.
// Lookup and apply are separate so the applier can perform its duplicate check,
// event insert, and conditional state transition in one transaction.
type Processor struct {
	lookup  ExecutionLookup
	applier EventApplier
	metrics *observability.Metrics
}

func NewProcessor(lookup ExecutionLookup, applier EventApplier) *Processor {
	return &Processor{lookup: lookup, applier: applier}
}

func (processor *Processor) WithMetrics(metrics *observability.Metrics) *Processor {
	processor.metrics = metrics
	return processor
}

func (processor *Processor) Process(ctx context.Context, event Event) (Decision, error) {
	if processor == nil || processor.lookup == nil || processor.applier == nil {
		return Conflict, fmt.Errorf("event processor is not configured")
	}
	if err := event.Validate(); err != nil {
		return Conflict, err
	}
	view, err := processor.lookup.LookupExternalExecution(ctx, event.ExternalExecutionID)
	if err != nil {
		return Conflict, fmt.Errorf("lookup external execution: %w", err)
	}
	if view.ID == "" || view.ProjectID == "" {
		return Conflict, fmt.Errorf("lookup external execution: incomplete local identity")
	}
	decision, err := processor.applier.ApplyExternalEvent(ctx, event, view.ProjectID, view.ID, view.Status, view.LastEventAt, time.Now())
	if decision == Duplicate && processor.metrics != nil {
		processor.metrics.WebhookDuplicates.Add(1)
	}
	if err != nil {
		return decision, fmt.Errorf("apply external execution event: %w", err)
	}
	return decision, nil
}
