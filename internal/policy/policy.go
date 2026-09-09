// Package policy contains the small deterministic policy decision used by the
// initial approval workflow. It is intentionally code-defined, not a DSL.
package policy

import (
	"fmt"

	"github.com/srikarjy/RunBridge/internal/preflight"
	"github.com/srikarjy/RunBridge/internal/rundiff"
)

const Version = "policy/v1"

type Outcome string

const (
	OutcomeAllow         Outcome = "allow"
	OutcomeRequireReview Outcome = "require_review"
	OutcomeDeny          Outcome = "deny"
)

type Evaluation struct {
	Version OutcomeVersion
	Outcome Outcome
	Reasons []string
}

type OutcomeVersion string

func Evaluate(preflightResult preflight.Result, diff rundiff.Result) Evaluation {
	evaluation := Evaluation{Version: Version, Outcome: OutcomeAllow, Reasons: []string{}}
	if !preflightResult.Ready() {
		evaluation.Outcome = OutcomeDeny
		for _, failure := range preflightResult.BlockingFailures() {
			evaluation.Reasons = append(evaluation.Reasons, fmt.Sprintf("preflight %s failed", failure.Code))
		}
		return evaluation
	}
	if diff.BaselineMissing {
		evaluation.Outcome = OutcomeRequireReview
		evaluation.Reasons = append(evaluation.Reasons, "first run has no baseline specification")
	} else if diff.Changed() {
		evaluation.Outcome = OutcomeRequireReview
		evaluation.Reasons = append(evaluation.Reasons, fmt.Sprintf("run diff contains %d change(s)", len(diff.Changes)))
	}
	return evaluation
}
