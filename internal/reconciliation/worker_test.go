package reconciliation

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerRunsImmediatelyAndStopsOnCancellation(t *testing.T) {
	var passes atomic.Int64
	source := candidateSourceFake{}
	observer := observerFake{}
	sink := &sinkFake{}
	service := NewService(source, observer, sink)
	worker, err := NewWorker(service, time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	service.source = countingSource{passes: &passes}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	deadline := time.After(time.Second)
	for passes.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("worker did not run immediate pass")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("worker exit: %v", err)
	}
}

type countingSource struct{ passes *atomic.Int64 }

func (source countingSource) ListCandidates(context.Context, int) ([]Candidate, error) {
	source.passes.Add(1)
	return nil, nil
}
