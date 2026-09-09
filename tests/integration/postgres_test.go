package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/srikarjy/RunBridge/internal/approvals"
	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/authorization"
	"github.com/srikarjy/RunBridge/internal/policy"
	"github.com/srikarjy/RunBridge/internal/postgres"
	"github.com/srikarjy/RunBridge/internal/preflight"
	"github.com/srikarjy/RunBridge/internal/projects"
	"github.com/srikarjy/RunBridge/internal/rundiff"
	"github.com/srikarjy/RunBridge/internal/runs"
)

func TestPostgresPersistence(t *testing.T) {
	databaseURL := os.Getenv("RUNBRIDGE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("RUNBRIDGE_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	schema := fmt.Sprintf("runbridge_test_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, `create schema `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	defer db.ExecContext(ctx, `drop schema `+schema+` cascade`)
	if _, err := db.ExecContext(ctx, `set search_path to `+schema); err != nil {
		t.Fatalf("set test schema: %v", err)
	}

	if err := postgres.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := postgres.Migrate(ctx, db); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	store := postgres.NewStore(db)
	actorID, _ := auth.NewActorID("actor-1")
	actor, _ := auth.NewActor(actorID, "Researcher", auth.ActorKindHuman)
	if err := store.CreateActor(ctx, actor); err != nil {
		t.Fatalf("create actor: %v", err)
	}

	projectID, _ := projects.NewProjectID("project-1")
	project, _ := projects.NewProject(projectID, "RNA sequencing")
	membership, _ := projects.NewMembership(projectID, actorID, projects.RoleRunner)
	if err := store.CreateProject(ctx, project, membership); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := authorization.NewAuthorizer(store).Authorize(ctx, actor, projectID, authorization.PermissionRunPropose); err != nil {
		t.Fatalf("authorize project runner: %v", err)
	}

	proposal := newProposal(t, projectID, actorID)
	if err := store.CreateProposal(ctx, proposal); err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	second := newSpecification(t, "spec-2", 2, actorID, `{"memory":"64 GB"}`)
	if err := store.AddSpecification(ctx, projectID, proposal.ID(), second); err != nil {
		t.Fatalf("add specification: %v", err)
	}

	loaded, err := store.GetProposal(ctx, projectID, proposal.ID())
	if err != nil {
		t.Fatalf("get proposal: %v", err)
	}
	if got := len(loaded.Specifications()); got != 2 {
		t.Fatalf("specification count = %d, want 2", got)
	}
	if got := string(loaded.LatestSpecification().Configuration().Document()); got != `{"memory":"64 GB"}` {
		t.Fatalf("normalized bytes = %q", got)
	}
	current := loaded.LatestSpecification()
	preflightResult := preflight.Result{Checks: []preflight.CheckResult{{Code: "ok", Status: preflight.StatusPass}}}
	diffResult := rundiff.Result{ProposedSpecificationID: current.ID(), Changes: []rundiff.Change{}}
	evaluation := policy.Evaluate(preflightResult, diffResult)
	approvalID, _ := approvals.NewApprovalID("approval-1")
	approval, err := approvals.Decide(approvalID, loaded, current, evaluation, approvals.ReviewContext{Preflight: preflightResult, Diff: diffResult}, "", false, true, time.Unix(4, 0).UTC())
	if err != nil {
		t.Fatalf("build policy approval: %v", err)
	}
	if err := store.CreateApproval(ctx, approval); err != nil {
		t.Fatalf("create approval: %v", err)
	}
	var decision, storedSpecificationID string
	if err := db.QueryRowContext(ctx, `select decision, specification_id from approvals where id = $1`, approvalID.String()).Scan(&decision, &storedSpecificationID); err != nil {
		t.Fatalf("read approval: %v", err)
	}
	if decision != string(approvals.DecisionPolicyApproved) || storedSpecificationID != current.ID().String() {
		t.Fatalf("stored approval = %s/%s", decision, storedSpecificationID)
	}
	if err := store.CreateExecution(ctx, "execution-1", projectID.String(), proposal.ID().String(), current.ID().String(), approval.ID().String(), runs.StatusApproved, time.Unix(5, 0).UTC()); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	if err := store.TransitionExecution(ctx, "execution-1", runs.StatusApproved, runs.StatusSubmitting, nil, nil); err != nil {
		t.Fatalf("transition execution: %v", err)
	}
	auditEvents, err := store.ListAuditEvents(ctx, projectID.String(), 20)
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(auditEvents) == 0 || auditEvents[len(auditEvents)-1].EventType != "execution.state_changed" {
		t.Fatalf("transition audit events = %#v", auditEvents)
	}

	if err := store.CreateExecutionWithWorkspace(ctx, "execution-2", projectID.String(), proposal.ID().String(), current.ID().String(), approval.ID().String(), runs.StatusApproved, "workspace-123", time.Unix(6, 0).UTC()); err != nil {
		t.Fatalf("create workspace execution: %v", err)
	}
	if err := store.ClaimSubmission(ctx, "execution-2", "attempt-1", "correlation-1", 1, time.Unix(7, 0).UTC()); err != nil {
		t.Fatalf("claim initial submission: %v", err)
	}
	if err := store.TransitionExecution(ctx, "execution-2", runs.StatusSubmitting, runs.StatusSubmissionUnknown, nil, nil); err != nil {
		t.Fatalf("mark submission unknown: %v", err)
	}
	candidates, err := store.ListCandidates(ctx, 10)
	if err != nil {
		t.Fatalf("list reconciliation candidates: %v", err)
	}
	found := false
	for _, candidate := range candidates {
		if candidate.ExecutionID == "execution-2" {
			found = candidate.WorkspaceID == "workspace-123" && candidate.CorrelationID == "correlation-1" && candidate.AttemptNumber == 1 && candidate.ExpectedStatus == runs.StatusSubmissionUnknown
		}
	}
	if !found {
		t.Fatalf("workspace execution not recoverable: %#v", candidates)
	}
	if err := store.ClaimRetrySubmission(ctx, "execution-2", "attempt-2", "correlation-2", 2, time.Unix(8, 0).UTC()); err != nil {
		t.Fatalf("claim retry submission: %v", err)
	}
	if err := store.ClaimRetrySubmission(ctx, "execution-2", "attempt-3", "correlation-3", 3, time.Unix(9, 0).UTC()); !errors.Is(err, postgres.ErrConflict) {
		t.Fatalf("duplicate retry claim error = %v, want ErrConflict", err)
	}
	var attemptCount int
	if err := db.QueryRowContext(ctx, `select count(*) from execution_attempts where execution_id = 'execution-2'`).Scan(&attemptCount); err != nil {
		t.Fatalf("count retry attempts: %v", err)
	}
	if attemptCount != 2 {
		t.Fatalf("attempt count = %d, want 2 after duplicate rollback", attemptCount)
	}

	wrongProjectID, _ := projects.NewProjectID("project-2")
	if _, err := store.GetProposal(ctx, wrongProjectID, proposal.ID()); !errors.Is(err, postgres.ErrNotFound) {
		t.Fatalf("cross-project lookup error = %v, want ErrNotFound", err)
	}
}

func TestCreateProjectRollsBackWhenMembershipFails(t *testing.T) {
	databaseURL := os.Getenv("RUNBRIDGE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("RUNBRIDGE_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	schema := fmt.Sprintf("runbridge_rollback_test_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, `create schema `+schema); err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, `drop schema `+schema+` cascade`)
	if _, err := db.ExecContext(ctx, `set search_path to `+schema); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	projectID, _ := projects.NewProjectID("project-rollback")
	project, _ := projects.NewProject(projectID, "Should roll back")
	missingActorID, _ := auth.NewActorID("missing-actor")
	membership, _ := projects.NewMembership(projectID, missingActorID, projects.RoleAdmin)
	err = postgres.NewStore(db).CreateProject(ctx, project, membership)
	if err == nil {
		t.Fatal("expected membership foreign-key failure")
	}
	var count int
	if err := db.QueryRowContext(ctx, `select count(*) from projects where id = $1`, projectID.String()).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("project count = %d, want transaction rollback", count)
	}
}

func newProposal(t *testing.T, projectID projects.ProjectID, actorID auth.ActorID) runs.Proposal {
	t.Helper()
	proposalID, _ := runs.NewProposalID("proposal-1")
	initial := newSpecification(t, "spec-1", 1, actorID, `{"memory":"16 GB"}`)
	proposal, err := runs.NewProposal(proposalID, projectID, actorID, time.Unix(1, 0).UTC(), initial)
	if err != nil {
		t.Fatal(err)
	}
	return proposal
}

func newSpecification(t *testing.T, idValue string, revision uint64, actorID auth.ActorID, document string) runs.Specification {
	t.Helper()
	id, _ := runs.NewSpecificationID(idValue)
	workflow, _ := runs.NewWorkflowIdentifier("nf-core/rnaseq", "3.14.0")
	configuration, _ := runs.NewNormalizedConfiguration("v1", []byte(document))
	specification, err := runs.NewSpecification(id, revision, workflow, configuration, actorID, time.Unix(int64(revision), 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	return specification
}
