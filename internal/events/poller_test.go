package events

import (
	"context"
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/runs"
)

type pollSourceFake struct{ candidates []PollCandidate }

func (source pollSourceFake) ListRunningExecutions(context.Context, int) ([]PollCandidate, error) {
	return source.candidates, nil
}

type statusObserverFake struct{ status runs.Status }

func (observer statusObserverFake) WorkflowStatus(context.Context, string) (runs.Status, error) {
	return observer.status, nil
}

type eventProcessorFake struct{ events []Event }

func (processor *eventProcessorFake) Process(_ context.Context, event Event) (Decision, error) {
	processor.events = append(processor.events, event)
	return Apply, nil
}

func TestPollerEmitsOnlyChangedStatus(t *testing.T) {
	source := pollSourceFake{candidates: []PollCandidate{{ExternalExecutionID: "wf-1", CurrentStatus: runs.StatusRunning}}}
	processor := &eventProcessorFake{}
	poller, err := NewPoller(source, statusObserverFake{status: runs.StatusSucceeded}, processor, time.Second, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := poller.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(processor.events) != 1 || processor.events[0].Status != runs.StatusSucceeded {
		t.Fatalf("events: %#v", processor.events)
	}

	processor.events = nil
	poller, _ = NewPoller(source, statusObserverFake{status: runs.StatusRunning}, processor, time.Second, 10)
	if err := poller.PollOnce(context.Background()); err != nil || len(processor.events) != 0 {
		t.Fatalf("unchanged poll: events=%#v err=%v", processor.events, err)
	}
}
