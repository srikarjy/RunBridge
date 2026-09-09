package policy_test

import (
	"testing"

	"github.com/srikarjy/RunBridge/internal/policy"
	"github.com/srikarjy/RunBridge/internal/preflight"
	"github.com/srikarjy/RunBridge/internal/rundiff"
)

func TestEvaluateDeniesPreflightFailures(t *testing.T) {
	result := policy.Evaluate(preflight.Result{Checks: []preflight.CheckResult{{Code: "resources.cpus.exceeded", Status: preflight.StatusFail}}}, rundiff.Result{})
	if result.Outcome != policy.OutcomeDeny || result.Version != policy.Version {
		t.Fatalf("unexpected evaluation: %#v", result)
	}
}

func TestEvaluateRequiresReviewForChangesAndFirstRun(t *testing.T) {
	passing := preflight.Result{Checks: []preflight.CheckResult{{Code: "ok", Status: preflight.StatusPass}}}
	changed := rundiff.Result{ProposedSpecificationID: "spec-2", Changes: []rundiff.Change{{Category: rundiff.CategoryResources, Path: "resources.cpus", Kind: rundiff.ChangeChanged}}}
	if result := policy.Evaluate(passing, changed); result.Outcome != policy.OutcomeRequireReview {
		t.Fatalf("changed evaluation = %#v", result)
	}
	if result := policy.Evaluate(passing, rundiff.Result{BaselineMissing: true}); result.Outcome != policy.OutcomeRequireReview {
		t.Fatalf("first-run evaluation = %#v", result)
	}
}

func TestEvaluateAllowsUnchangedPassingRun(t *testing.T) {
	passing := preflight.Result{Checks: []preflight.CheckResult{{Code: "ok", Status: preflight.StatusPass}}}
	if result := policy.Evaluate(passing, rundiff.Result{}); result.Outcome != policy.OutcomeAllow {
		t.Fatalf("unchanged evaluation = %#v", result)
	}
}
