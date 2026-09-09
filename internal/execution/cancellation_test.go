package execution

import (
	"context"
	"testing"

	"github.com/srikarjy/RunBridge/internal/runs"
)

type fakeCanceller struct{ called bool }

func (canceller *fakeCanceller) Cancel(context.Context, string) error {
	canceller.called = true
	return nil
}

func TestRequestCancellationWaitsForRemoteConfirmation(t *testing.T) {
	store := &fakeCoordinatorStore{}
	canceller := &fakeCanceller{}
	coordinator := NewCoordinator(store, store, fakeLauncher{})
	if err := coordinator.RequestCancellation(context.Background(), runs.StatusRunning, "exec-1", "wf-1", canceller); err != nil {
		t.Fatal(err)
	}
	if !canceller.called || store.next != "" {
		t.Fatalf("remote cancellation changed local state: %#v", store)
	}
}
