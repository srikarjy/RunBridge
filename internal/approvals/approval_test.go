package approvals_test

import (
	"errors"
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/approvals"
	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/policy"
	"github.com/srikarjy/RunBridge/internal/preflight"
	"github.com/srikarjy/RunBridge/internal/projects"
	"github.com/srikarjy/RunBridge/internal/rundiff"
	"github.com/srikarjy/RunBridge/internal/runs"
	"github.com/srikarjy/RunBridge/internal/runs/rnaseq"
)

func TestDecideBindsReviewerApprovalToExactRevision(t *testing.T) {
	proposal, specification, actorID := proposalFixture(t)
	result := preflight.Result{Checks: []preflight.CheckResult{{Code: "configuration.canonical", Status: preflight.StatusPass}}}
	before, after := "8", "16"
	diff := rundiff.Result{BaselineSpecificationID: "baseline", ProposedSpecificationID: specification.ID(), Changes: []rundiff.Change{{Category: rundiff.CategoryResources, Path: "resources.cpus", Kind: rundiff.ChangeChanged, Before: &before, After: &after}}}
	evaluation := policy.Evaluate(result, diff)
	approvalID, _ := approvals.NewApprovalID("approval-1")
	approval, err := approvals.Decide(approvalID, proposal, specification, evaluation, approvals.ReviewContext{Preflight: result, Diff: diff}, actorID, true, true, time.Unix(3, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if approval.Decision() != approvals.DecisionApproved || !approval.IsCurrent(proposal) {
		t.Fatalf("unexpected approval: %s", approval)
	}
	changedSpecification := specificationFixture(t, actorID, "spec-2", 2, "3.15.1")
	if err := proposal.AddSpecification(changedSpecification); err != nil {
		t.Fatal(err)
	}
	if approval.IsCurrent(proposal) {
		t.Fatal("approval remained current after proposal revision")
	}
	context := approval.ReviewContext()
	context.Diff.Changes[0].Path = "mutated"
	if approval.ReviewContext().Diff.Changes[0].Path == "mutated" {
		t.Fatal("approval context was mutable")
	}
	*context.Diff.Changes[0].Before = "mutated-value"
	if *approval.ReviewContext().Diff.Changes[0].Before == "mutated-value" {
		t.Fatal("approval context value was mutable")
	}
}

func TestDecideCreatesPolicyApprovalWithoutReviewer(t *testing.T) {
	proposal, specification, _ := proposalFixture(t)
	result := preflight.Result{Checks: []preflight.CheckResult{{Code: "ok", Status: preflight.StatusPass}}}
	diff := rundiff.Result{ProposedSpecificationID: specification.ID(), Changes: []rundiff.Change{}}
	evaluation := policy.Evaluate(result, diff)
	approvalID, _ := approvals.NewApprovalID("approval-1")
	approval, err := approvals.Decide(approvalID, proposal, specification, evaluation, approvals.ReviewContext{Preflight: result, Diff: diff}, "", false, true, time.Unix(3, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if approval.Decision() != approvals.DecisionPolicyApproved {
		t.Fatalf("decision = %q", approval.Decision())
	}
	if _, hasReviewer := approval.Reviewer(); hasReviewer {
		t.Fatal("policy approval has reviewer")
	}
}

func TestDecideRejectsDeniedOrUnreviewedChanges(t *testing.T) {
	proposal, specification, actorID := proposalFixture(t)
	failing := preflight.Result{Checks: []preflight.CheckResult{{Code: "bad", Status: preflight.StatusFail}}}
	denied := policy.Evaluate(failing, rundiff.Result{ProposedSpecificationID: specification.ID()})
	approvalID, _ := approvals.NewApprovalID("approval-1")
	_, err := approvals.Decide(approvalID, proposal, specification, denied, approvals.ReviewContext{Preflight: failing, Diff: rundiff.Result{ProposedSpecificationID: specification.ID()}}, actorID, true, true, time.Unix(3, 0).UTC())
	if !errors.Is(err, approvals.ErrDecisionDenied) {
		t.Fatalf("denied error = %v", err)
	}
	changed := rundiff.Result{ProposedSpecificationID: specification.ID(), Changes: []rundiff.Change{{Category: rundiff.CategoryInputs, Path: "samples[x]", Kind: rundiff.ChangeAdded}}}
	requiresReview := policy.Evaluate(preflight.Result{Checks: []preflight.CheckResult{{Code: "ok", Status: preflight.StatusPass}}}, changed)
	_, err = approvals.Decide(approvalID, proposal, specification, requiresReview, approvals.ReviewContext{Preflight: preflight.Result{Checks: []preflight.CheckResult{{Code: "ok", Status: preflight.StatusPass}}}, Diff: changed}, "", false, true, time.Unix(3, 0).UTC())
	if !errors.Is(err, approvals.ErrReviewerRequired) {
		t.Fatalf("missing reviewer error = %v", err)
	}
}

func TestDecideRejectsFalseApprovalForAllowedPolicy(t *testing.T) {
	proposal, specification, _ := proposalFixture(t)
	passing := preflight.Result{Checks: []preflight.CheckResult{{Code: "ok", Status: preflight.StatusPass}}}
	evaluation := policy.Evaluate(passing, rundiff.Result{ProposedSpecificationID: specification.ID()})
	approvalID, _ := approvals.NewApprovalID("approval-1")
	_, err := approvals.Decide(approvalID, proposal, specification, evaluation, approvals.ReviewContext{Preflight: passing, Diff: rundiff.Result{ProposedSpecificationID: specification.ID()}}, "", false, false, time.Unix(3, 0).UTC())
	if !errors.Is(err, approvals.ErrInvalidDecision) {
		t.Fatalf("invalid decision error = %v", err)
	}
}

func proposalFixture(t *testing.T) (runs.Proposal, runs.Specification, auth.ActorID) {
	t.Helper()
	actorID, _ := auth.NewActorID("actor-1")
	projectID, _ := projects.NewProjectID("project-1")
	proposalID, _ := runs.NewProposalID("proposal-1")
	specification := specificationFixture(t, actorID, "spec-1", 1, "3.14.0")
	proposal, err := runs.NewProposal(proposalID, projectID, actorID, time.Unix(1, 0).UTC(), specification)
	if err != nil {
		t.Fatal(err)
	}
	return proposal, specification, actorID
}

func specificationFixture(t *testing.T, actorID auth.ActorID, idValue string, revision uint64, workflowRevision string) runs.Specification {
	t.Helper()
	workflow, configuration, err := rnaseq.Normalize(rnaseq.Request{WorkflowRevision: workflowRevision, Samples: []rnaseq.SampleInput{{ID: "sample-a", Read1: "a_1", Read2: "a_2"}}, Reference: rnaseq.ReferenceInput{Genome: "GRCh38", Fasta: "ref.fa"}, Profile: "docker", Resources: rnaseq.ResourceRequest{CPUs: 8, MemoryMiB: 16384}})
	if err != nil {
		t.Fatal(err)
	}
	id, _ := runs.NewSpecificationID(idValue)
	specification, err := runs.NewSpecification(id, revision, workflow, configuration, actorID, time.Unix(int64(revision), 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	return specification
}
