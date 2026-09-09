// Package approvals defines immutable decisions bound to exact run intent.
package approvals

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/policy"
	"github.com/srikarjy/RunBridge/internal/preflight"
	"github.com/srikarjy/RunBridge/internal/projects"
	"github.com/srikarjy/RunBridge/internal/rundiff"
	"github.com/srikarjy/RunBridge/internal/runs"
)

var (
	ErrApprovalIDRequired         = errors.New("approval ID is required")
	ErrDecisionDenied             = errors.New("denied policy result cannot be approved")
	ErrReviewerRequired           = errors.New("reviewer is required for this policy result")
	ErrReviewerUnexpected         = errors.New("reviewer is not allowed for this policy result")
	ErrApprovalContextMismatch    = errors.New("approval context does not match policy evaluation")
	ErrSpecificationNotInProposal = errors.New("specification is not part of proposal")
	ErrApprovalTimeRequired       = errors.New("approval decision time is required")
	ErrInvalidDecision            = errors.New("decision does not match policy outcome")
)

type ApprovalID string

func NewApprovalID(value string) (ApprovalID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrApprovalIDRequired
	}
	return ApprovalID(value), nil
}

func (id ApprovalID) String() string { return string(id) }

type Decision string

const (
	DecisionApproved       Decision = "approved"
	DecisionRejected       Decision = "rejected"
	DecisionPolicyApproved Decision = "policy_approved"
)

// ReviewContext is the evidence a decision was based on. The values are
// copied into Approval so callers cannot mutate a recorded decision through a
// shared slice.
type ReviewContext struct {
	Preflight preflight.Result `json:"preflight"`
	Diff      rundiff.Result   `json:"diff"`
}

type Approval struct {
	id                    ApprovalID
	projectID             projects.ProjectID
	proposalID            runs.ProposalID
	specificationID       runs.SpecificationID
	specificationRevision uint64
	reviewerID            auth.ActorID
	hasReviewer           bool
	decision              Decision
	policyVersion         policy.OutcomeVersion
	context               ReviewContext
	decidedAt             time.Time
}

// Decide binds a policy decision to one immutable proposal specification. A
// denied preflight cannot be converted into an approval; review is required
// only for the policy's require_review outcome.
func Decide(id ApprovalID, proposal runs.Proposal, specification runs.Specification, evaluation policy.Evaluation, context ReviewContext, reviewer auth.ActorID, hasReviewer bool, approve bool, decidedAt time.Time) (Approval, error) {
	if strings.TrimSpace(id.String()) == "" {
		return Approval{}, ErrApprovalIDRequired
	}
	if !containsSpecification(proposal, specification.ID(), specification.Revision()) {
		return Approval{}, ErrSpecificationNotInProposal
	}
	if evaluation.Outcome == policy.OutcomeDeny {
		return Approval{}, ErrDecisionDenied
	}
	if evaluation.Version == "" || evaluation.Version != policy.Version {
		return Approval{}, ErrApprovalContextMismatch
	}
	if context.Diff.ProposedSpecificationID != specification.ID() {
		return Approval{}, ErrApprovalContextMismatch
	}
	if context.Preflight.Ready() != (evaluation.Outcome != policy.OutcomeDeny) {
		return Approval{}, ErrApprovalContextMismatch
	}
	if decidedAt.IsZero() {
		return Approval{}, ErrApprovalTimeRequired
	}
	if evaluation.Outcome == policy.OutcomeRequireReview {
		if !hasReviewer || strings.TrimSpace(reviewer.String()) == "" {
			return Approval{}, ErrReviewerRequired
		}
	} else if hasReviewer || strings.TrimSpace(reviewer.String()) != "" {
		return Approval{}, ErrReviewerUnexpected
	} else if !approve {
		return Approval{}, ErrInvalidDecision
	}
	decision := DecisionPolicyApproved
	if evaluation.Outcome == policy.OutcomeRequireReview {
		decision = DecisionRejected
		if approve {
			decision = DecisionApproved
		}
	}
	return Approval{id: id, projectID: proposal.ProjectID(), proposalID: proposal.ID(), specificationID: specification.ID(), specificationRevision: specification.Revision(), reviewerID: reviewer, hasReviewer: hasReviewer, decision: decision, policyVersion: evaluation.Version, context: cloneContext(context), decidedAt: decidedAt}, nil
}

func containsSpecification(proposal runs.Proposal, id runs.SpecificationID, revision uint64) bool {
	for _, specification := range proposal.Specifications() {
		if specification.ID() == id && specification.Revision() == revision {
			return true
		}
	}
	return false
}

func (approval Approval) ID() ApprovalID                        { return approval.id }
func (approval Approval) ProjectID() projects.ProjectID         { return approval.projectID }
func (approval Approval) ProposalID() runs.ProposalID           { return approval.proposalID }
func (approval Approval) SpecificationID() runs.SpecificationID { return approval.specificationID }
func (approval Approval) SpecificationRevision() uint64         { return approval.specificationRevision }
func (approval Approval) Decision() Decision                    { return approval.decision }
func (approval Approval) PolicyVersion() policy.OutcomeVersion  { return approval.policyVersion }
func (approval Approval) DecidedAt() time.Time                  { return approval.decidedAt }
func (approval Approval) Reviewer() (auth.ActorID, bool) {
	return approval.reviewerID, approval.hasReviewer
}
func (approval Approval) ReviewContext() ReviewContext { return cloneContext(approval.context) }

// IsCurrent reports whether the approval still targets the proposal's latest
// revision. Older approvals remain historical evidence but cannot authorize a
// changed proposal.
func (approval Approval) IsCurrent(proposal runs.Proposal) bool {
	if len(proposal.Specifications()) == 0 {
		return false
	}
	latest := proposal.LatestSpecification()
	return approval.proposalID == proposal.ID() && latest.ID() == approval.specificationID && latest.Revision() == approval.specificationRevision
}

func cloneContext(context ReviewContext) ReviewContext {
	cloned := ReviewContext{Preflight: context.Preflight, Diff: context.Diff}
	cloned.Preflight.Checks = append([]preflight.CheckResult(nil), context.Preflight.Checks...)
	cloned.Diff.Changes = cloneChanges(context.Diff.Changes)
	return cloned
}

func cloneChanges(changes []rundiff.Change) []rundiff.Change {
	cloned := make([]rundiff.Change, len(changes))
	for index, change := range changes {
		cloned[index] = change
		if change.Before != nil {
			before := *change.Before
			cloned[index].Before = &before
		}
		if change.After != nil {
			after := *change.After
			cloned[index].After = &after
		}
	}
	return cloned
}

func (approval Approval) String() string {
	return fmt.Sprintf("%s (%s, specification %s)", approval.id, approval.decision, approval.specificationID)
}
