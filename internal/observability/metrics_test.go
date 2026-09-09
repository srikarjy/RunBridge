package observability

import "testing"

func TestMetricsSnapshot(t *testing.T) {
	var metrics Metrics
	metrics.SubmissionFailures.Add(2)
	metrics.ReconciliationAttempts.Add(3)
	metrics.WebhookDuplicates.Add(4)
	metrics.TransitionConflicts.Add(5)
	if got := metrics.Snapshot(); got != (Snapshot{2, 3, 4, 5}) {
		t.Fatalf("snapshot: %#v", got)
	}
}
