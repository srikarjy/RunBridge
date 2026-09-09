// Package reconciliation defines decisions for ambiguous external submissions.
// It does not query Seqera or mutate execution state; a coordinator supplies
// observations and persists the returned decision transactionally.
package reconciliation

import "errors"

type Observation struct {
	ExternalID string
	Status     string
}

type DecisionKind string

const (
	AdoptExecution DecisionKind = "adopt_execution"
	RetryAllowed   DecisionKind = "retry_allowed"
	RemainUnknown  DecisionKind = "remain_unknown"
	ManualReview   DecisionKind = "manual_review"
)

var ErrMultipleMatches = errors.New("multiple external executions match submission")
var ErrRetryLimit = errors.New("submission retry limit reached")

type Decision struct {
	Kind       DecisionKind
	ExternalID string
}

// Decide returns a conservative result. An empty search is not proof that a
// launch was rejected because external systems may be eventually consistent.
func Decide(observations []Observation, definitiveFailure bool) Decision {
	switch len(observations) {
	case 0:
		if definitiveFailure {
			return Decision{Kind: RetryAllowed}
		}
		return Decision{Kind: RemainUnknown}
	case 1:
		return Decision{Kind: AdoptExecution, ExternalID: observations[0].ExternalID}
	default:
		return Decision{Kind: ManualReview}
	}
}

func CanRetry(attemptNumber, maxAttempts int64, definitiveFailure bool) error {
	if !definitiveFailure {
		return ErrRetryLimit
	}
	if attemptNumber <= 0 || maxAttempts <= 0 || attemptNumber >= maxAttempts {
		return ErrRetryLimit
	}
	return nil
}
