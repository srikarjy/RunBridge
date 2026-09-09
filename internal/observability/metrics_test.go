package observability

import "testing"

func TestMetricsSnapshot(t *testing.T) {
	var metrics Metrics
	metrics.SubmissionFailures.Add(2)
	metrics.ReconciliationAttempts.Add(3)
	metrics.WebhookDuplicates.Add(4)
	metrics.TransitionConflicts.Add(5)
	if got := metrics.Snapshot(); got != (Snapshot{SubmissionFailures: 2, ReconciliationAttempts: 3, WebhookDuplicates: 4, TransitionConflicts: 5}) {
		t.Fatalf("snapshot: %#v", got)
	}
}
