package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/runs"
)

type fakeCoordinatorStore struct {
	attempt, transition int
	next                runs.Status
	external            string
}

func (store *fakeCoordinatorStore) CreateAttempt(context.Context, string, string, string, int64, time.Time) error {
	store.attempt++
	return nil
}
func (store *fakeCoordinatorStore) ClaimRetrySubmission(context.Context, string, string, string, int64, time.Time) error {
	store.attempt++
	store.next = runs.StatusSubmitting
	return nil
}
func (store *fakeCoordinatorStore) TransitionExecution(_ context.Context, _ string, _, next runs.Status, _, external *string) error {
	store.transition++
	store.next = next
	if external != nil {
		store.external = *external
	}
	return nil
}

type fakeLauncher struct {
	response LaunchResponse
	err      error
	request  *LaunchRequest
}

type auditCoordinatorStore struct {
	fakeCoordinatorStore
	audited bool
}

func (store *auditCoordinatorStore) RecordSubmissionAttempt(context.Context, string, string, string, int64, time.Time) error {
	store.audited = true
	return nil
}

func (launcher fakeLauncher) Submit(_ context.Context, request LaunchRequest) (LaunchResponse, error) {
	if launcher.request != nil {
		*launcher.request = request
	}
	return launcher.response, launcher.err
}

func TestCoordinatorMarksAcceptedSubmissionRunning(t *testing.T) {
	store := &fakeCoordinatorStore{}
	coordinator := NewCoordinator(store, store, fakeLauncher{response: LaunchResponse{ExternalExecutionID: "wf-1"}})
	status, id, err := coordinator.Submit(context.Background(), "exec-1", "attempt-1", "corr-1", 1, LaunchRequest{WorkspaceID: "123", Pipeline: "nf-core/rnaseq", Revision: "3.18.0"}, time.Now().UTC())
	if err != nil || status != runs.StatusRunning || id != "wf-1" || store.transition != 2 {
		t.Fatalf("accepted submission: %s %q %v %#v", status, id, err, store)
	}
}

func TestCoordinatorPreservesUncertainty(t *testing.T) {
	store := &fakeCoordinatorStore{}
	coordinator := NewCoordinator(store, store, fakeLauncher{err: errors.New("timeout")})
	status, _, err := coordinator.Submit(context.Background(), "exec-1", "attempt-1", "corr-1", 1, LaunchRequest{}, time.Now().UTC())
	if status != runs.StatusSubmissionUnknown || !errors.Is(err, ErrSubmissionUncertain) {
		t.Fatalf("uncertain submission: %s %v", status, err)
	}
}

func TestCoordinatorCancelsOnlyApprovedExecution(t *testing.T) {
	store := &fakeCoordinatorStore{}
	if err := NewCoordinator(store, store, fakeLauncher{}).CancelBeforeSubmission(context.Background(), "exec-1"); err != nil {
		t.Fatal(err)
	}
	if store.next != runs.StatusCancelled {
		t.Fatalf("status: %s", store.next)
	}
}

func TestCoordinatorSubmitsDurableRetry(t *testing.T) {
	store := &fakeCoordinatorStore{}
	coordinator := NewCoordinator(store, store, fakeLauncher{response: LaunchResponse{ExternalExecutionID: "wf-retry"}})
	status, id, err := coordinator.SubmitRetry(context.Background(), "exec-1", "attempt-2", "corr-2", 2, LaunchRequest{WorkspaceID: "123", Pipeline: "nf-core/rnaseq", Revision: "3.18.0"}, time.Now())
	if err != nil || status != runs.StatusRunning || id != "wf-retry" || store.attempt != 1 {
		t.Fatalf("retry submission: status=%s id=%q err=%v store=%#v", status, id, err, store)
	}
}

func TestCoordinatorRecordsAttemptAuditBeforeLaunch(t *testing.T) {
	store := &auditCoordinatorStore{}
	coordinator := NewCoordinator(store, store, fakeLauncher{response: LaunchResponse{ExternalExecutionID: "wf-audited"}})
	status, _, err := coordinator.Submit(context.Background(), "exec-1", "attempt-1", "corr-1", 1, LaunchRequest{WorkspaceID: "123", Pipeline: "nf-core/rnaseq", Revision: "3.18.0"}, time.Now())
	if err != nil || status != runs.StatusRunning || !store.audited {
		t.Fatalf("audit before launch: status=%s err=%v audited=%v", status, err, store.audited)
	}
}

func TestCoordinatorBindsCorrelationToExternalRunName(t *testing.T) {
	store := &fakeCoordinatorStore{}
	var submitted LaunchRequest
	coordinator := NewCoordinator(store, store, fakeLauncher{response: LaunchResponse{ExternalExecutionID: "wf-1"}, request: &submitted})
	_, _, err := coordinator.Submit(context.Background(), "exec-1", "attempt-1", "sensitive-correlation", 1, LaunchRequest{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	want := CorrelationRunName("sensitive-correlation")
	if submitted.RunName != want || submitted.RunName == "sensitive-correlation" {
		t.Fatalf("run name = %q, want %q", submitted.RunName, want)
	}
}
