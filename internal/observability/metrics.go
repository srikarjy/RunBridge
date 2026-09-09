package observability

import "sync/atomic"

type Metrics struct {
	HTTPRequests           atomic.Uint64
	HTTPErrors             atomic.Uint64
	SubmissionFailures     atomic.Uint64
	ReconciliationAttempts atomic.Uint64
	WebhookDuplicates      atomic.Uint64
	TransitionConflicts    atomic.Uint64
}

type Snapshot struct {
	HTTPRequests           uint64
	HTTPErrors             uint64
	SubmissionFailures     uint64
	ReconciliationAttempts uint64
	WebhookDuplicates      uint64
	TransitionConflicts    uint64
}

func (metrics *Metrics) Snapshot() Snapshot {
	return Snapshot{
		HTTPRequests:           metrics.HTTPRequests.Load(),
		HTTPErrors:             metrics.HTTPErrors.Load(),
		SubmissionFailures:     metrics.SubmissionFailures.Load(),
		ReconciliationAttempts: metrics.ReconciliationAttempts.Load(),
		WebhookDuplicates:      metrics.WebhookDuplicates.Load(),
		TransitionConflicts:    metrics.TransitionConflicts.Load(),
	}
}
