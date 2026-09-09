package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/srikarjy/RunBridge/internal/approvals"
	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/execution"
	"github.com/srikarjy/RunBridge/internal/projects"
	"github.com/srikarjy/RunBridge/internal/runs"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record conflicts with existing data")
	ErrNotDraft = errors.New("proposal is not editable")
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (store *Store) CreateActor(ctx context.Context, actor auth.Actor) error {
	_, err := store.db.ExecContext(ctx, `
        insert into actors (id, display_name, kind)
        values ($1, $2, $3)
    `, actor.ID().String(), actor.DisplayName(), actor.Kind())
	return classify("create actor", err)
}

// CreateProject saves a project and its initial membership atomically.
func (store *Store) CreateProject(ctx context.Context, project projects.Project, membership projects.Membership) error {
	if project.ID() != membership.ProjectID() {
		return fmt.Errorf("create project: membership project does not match: %w", ErrConflict)
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create project: begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `insert into projects (id, name) values ($1, $2)`, project.ID().String(), project.Name()); err != nil {
		return classify("create project", err)
	}
	if _, err := tx.ExecContext(ctx, `
        insert into project_memberships (project_id, actor_id, role)
        values ($1, $2, $3)
    `, membership.ProjectID().String(), membership.ActorID().String(), membership.Role()); err != nil {
		return classify("create project membership", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create project: commit: %w", err)
	}
	return nil
}

func (store *Store) AddMembership(ctx context.Context, membership projects.Membership) error {
	_, err := store.db.ExecContext(ctx, `
        insert into project_memberships (project_id, actor_id, role)
        values ($1, $2, $3)
    `, membership.ProjectID().String(), membership.ActorID().String(), membership.Role())
	return classify("add project membership", err)
}

func (store *Store) MembershipFor(ctx context.Context, projectID projects.ProjectID, actorID auth.ActorID) (projects.Membership, error) {
	var role projects.Role
	err := store.db.QueryRowContext(ctx, `
        select role
        from project_memberships
        where project_id = $1 and actor_id = $2
    `, projectID.String(), actorID.String()).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return projects.Membership{}, fmt.Errorf("get project membership: %w: %w", ErrNotFound, projects.ErrMembershipNotFound)
	}
	if err != nil {
		return projects.Membership{}, fmt.Errorf("get project membership: %w", err)
	}
	membership, err := projects.NewMembership(projectID, actorID, role)
	if err != nil {
		return projects.Membership{}, fmt.Errorf("rebuild project membership: %w", err)
	}
	return membership, nil
}

// CreateProposal stores a proposal and all of its immutable revisions in one
// short transaction.
func (store *Store) CreateProposal(ctx context.Context, proposal runs.Proposal) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create proposal: begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
        insert into run_proposals (id, project_id, created_by, status, created_at)
        values ($1, $2, $3, $4, $5)
    `, proposal.ID().String(), proposal.ProjectID().String(), proposal.CreatedBy().String(), proposal.Status(), proposal.CreatedAt()); err != nil {
		return classify("create proposal", err)
	}
	for _, specification := range proposal.Specifications() {
		if err := insertSpecification(ctx, tx, proposal.ID(), specification); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create proposal: commit: %w", err)
	}
	return nil
}

// AddSpecification persists the next draft revision. Locking the proposal row
// serializes revision allocation without holding a transaction across a
// network call.
func (store *Store) AddSpecification(ctx context.Context, projectID projects.ProjectID, proposalID runs.ProposalID, specification runs.Specification) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("add specification: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var status runs.Status
	if err := tx.QueryRowContext(ctx, `
        select status
        from run_proposals
        where id = $1 and project_id = $2
        for update
    `, proposalID.String(), projectID.String()).Scan(&status); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("add specification: lock proposal: %w", err)
	}
	if status != runs.StatusDraft {
		return ErrNotDraft
	}

	var nextRevision uint64
	if err := tx.QueryRowContext(ctx, `
        select coalesce(max(revision), 0) + 1
        from run_specifications
        where proposal_id = $1
    `, proposalID.String()).Scan(&nextRevision); err != nil {
		return fmt.Errorf("add specification: read next revision: %w", err)
	}
	if specification.Revision() != nextRevision {
		return fmt.Errorf("add specification: expected revision %d: %w", nextRevision, ErrConflict)
	}
	if err := insertSpecification(ctx, tx, proposalID, specification); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("add specification: commit: %w", err)
	}
	return nil
}

// CreateApproval persists the immutable decision and the review context that
// produced it. The approved specification itself is referenced by ID; it is
// never copied from the latest proposal pointer.
func (store *Store) CreateApproval(ctx context.Context, approval approvals.Approval) error {
	contextBytes, err := json.Marshal(approval.ReviewContext())
	if err != nil {
		return fmt.Errorf("create approval: marshal review context: %w", err)
	}
	var reviewer any
	if reviewerID, ok := approval.Reviewer(); ok {
		reviewer = reviewerID.String()
	}
	_, err = store.db.ExecContext(ctx, `
        insert into approvals (
            id, project_id, proposal_id, specification_id, reviewer_id,
            decision, policy_version, review_context, decided_at
        ) values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
    `,
		approval.ID().String(), approval.ProjectID().String(), approval.ProposalID().String(), approval.SpecificationID().String(),
		reviewer, approval.Decision(), approval.PolicyVersion(), string(contextBytes), approval.DecidedAt(),
	)
	return classify("create approval", err)
}

// CreateExecution persists the approved intent before any external launch.
// The approval/specification foreign keys ensure the execution cannot point
// at an unrelated or mutable proposal revision.
func (store *Store) CreateExecution(ctx context.Context, id, projectID, proposalID, specificationID, approvalID string, status runs.Status, createdAt time.Time) error {
	if err := execution.ValidateState(status); err != nil {
		return fmt.Errorf("create execution: %w", err)
	}
	if status != runs.StatusApproved {
		return fmt.Errorf("create execution: initial state must be APPROVED: %w", ErrConflict)
	}
	if createdAt.IsZero() {
		return fmt.Errorf("create execution: created time is required: %w", ErrConflict)
	}
	_, err := store.db.ExecContext(ctx, `
        insert into executions (id, project_id, proposal_id, specification_id, approval_id, status, created_at, updated_at)
        values ($1, $2, $3, $4, $5, $6, $7, $7)
    `, id, projectID, proposalID, specificationID, approvalID, status, createdAt)
	return classify("create execution", err)
}

// TransitionExecution performs a conditional update. A stale expected state
// cannot overwrite a concurrent coordinator or reconciler decision.
func (store *Store) TransitionExecution(ctx context.Context, id string, expected, next runs.Status, externalWorkspaceID, externalExecutionID *string) error {
	if err := execution.Transition(expected, next); err != nil {
		return err
	}
	if (externalWorkspaceID == nil) != (externalExecutionID == nil) {
		return fmt.Errorf("transition execution: external identifiers must be supplied together: %w", ErrConflict)
	}
	result, err := store.db.ExecContext(ctx, `
        update executions
        set status = $1, external_workspace_id = coalesce($2, external_workspace_id),
            external_execution_id = coalesce($3, external_execution_id), updated_at = now()
        where id = $4 and status = $5
    `, next, nullableString(externalWorkspaceID), nullableString(externalExecutionID), id, expected)
	if err != nil {
		return classify("transition execution", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("transition execution: rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("transition execution: expected %s for %s: %w", expected, id, ErrConflict)
	}
	return nil
}

// CreateAttempt records one transport attempt. The database uniqueness rules
// make attempt numbers and correlation IDs safe against concurrent retries.
func (store *Store) CreateAttempt(ctx context.Context, id, executionID, correlationID string, number int64, startedAt time.Time) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(executionID) == "" || strings.TrimSpace(correlationID) == "" || number <= 0 || startedAt.IsZero() {
		return fmt.Errorf("create execution attempt: invalid identity or timestamp: %w", ErrConflict)
	}
	_, err := store.db.ExecContext(ctx, `
        insert into execution_attempts (id, execution_id, attempt_number, correlation_id, status, started_at)
        values ($1, $2, $3, $4, 'pending', $5)
    `, id, executionID, number, correlationID, startedAt)
	return classify("create execution attempt", err)
}

// ResolveAttempt closes a pending/unknown attempt exactly once. A repeated
// resolution is reported as a conflict so callers cannot silently rewrite
// evidence used by reconciliation.
func (store *Store) ResolveAttempt(ctx context.Context, executionID string, number int64, status execution.AttemptStatus, errorCategory *string, resolvedAt time.Time) error {
	if strings.TrimSpace(executionID) == "" || number <= 0 || !status.Valid() || status == execution.AttemptPending || resolvedAt.IsZero() {
		return fmt.Errorf("resolve execution attempt: invalid resolution: %w", ErrConflict)
	}
	result, err := store.db.ExecContext(ctx, `
        update execution_attempts
        set status = $1, last_error_category = $2, resolved_at = $3
        where execution_id = $4 and attempt_number = $5
          and status in ('pending', 'unknown') and resolved_at is null
    `, status, nullableString(errorCategory), resolvedAt, executionID, number)
	if err != nil {
		return classify("resolve execution attempt", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("resolve execution attempt: rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("resolve execution attempt: no unresolved attempt %d for %s: %w", number, executionID, ErrConflict)
	}
	return nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func (store *Store) GetProposal(ctx context.Context, projectID projects.ProjectID, proposalID runs.ProposalID) (runs.Proposal, error) {
	var createdBy string
	var status runs.Status
	var createdAt time.Time
	err := store.db.QueryRowContext(ctx, `
        select created_by, status, created_at
        from run_proposals
        where id = $1 and project_id = $2
    `, proposalID.String(), projectID.String()).Scan(&createdBy, &status, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return runs.Proposal{}, ErrNotFound
	}
	if err != nil {
		return runs.Proposal{}, fmt.Errorf("get proposal: %w", err)
	}
	if status != runs.StatusDraft {
		return runs.Proposal{}, fmt.Errorf("get proposal: unsupported persisted status %q", status)
	}

	actorID, err := auth.NewActorID(createdBy)
	if err != nil {
		return runs.Proposal{}, fmt.Errorf("get proposal creator: %w", err)
	}
	specifications, err := store.getSpecifications(ctx, proposalID)
	if err != nil {
		return runs.Proposal{}, err
	}
	if len(specifications) == 0 {
		return runs.Proposal{}, fmt.Errorf("get proposal: no specifications: %w", ErrNotFound)
	}

	proposal, err := runs.NewProposal(proposalID, projectID, actorID, createdAt, specifications[0])
	if err != nil {
		return runs.Proposal{}, fmt.Errorf("rebuild proposal: %w", err)
	}
	for _, specification := range specifications[1:] {
		if err := proposal.AddSpecification(specification); err != nil {
			return runs.Proposal{}, fmt.Errorf("rebuild proposal: %w", err)
		}
	}
	return proposal, nil
}

func (store *Store) getSpecifications(ctx context.Context, proposalID runs.ProposalID) ([]runs.Specification, error) {
	rows, err := store.db.QueryContext(ctx, `
        select id, revision, workflow_name, workflow_revision,
               normalization_version, normalized_document, created_by, created_at
        from run_specifications
        where proposal_id = $1
        order by revision
    `, proposalID.String())
	if err != nil {
		return nil, fmt.Errorf("get specifications: %w", err)
	}
	defer rows.Close()

	var specifications []runs.Specification
	for rows.Next() {
		var idValue, workflowName, workflowRevision, normalizationVersion, createdByValue string
		var revision uint64
		var document []byte
		var createdAt time.Time
		if err := rows.Scan(&idValue, &revision, &workflowName, &workflowRevision, &normalizationVersion, &document, &createdByValue, &createdAt); err != nil {
			return nil, fmt.Errorf("scan specification: %w", err)
		}
		id, err := runs.NewSpecificationID(idValue)
		if err != nil {
			return nil, fmt.Errorf("rebuild specification ID: %w", err)
		}
		workflow, err := runs.NewWorkflowIdentifier(workflowName, workflowRevision)
		if err != nil {
			return nil, fmt.Errorf("rebuild workflow: %w", err)
		}
		configuration, err := runs.NewNormalizedConfiguration(normalizationVersion, document)
		if err != nil {
			return nil, fmt.Errorf("rebuild configuration: %w", err)
		}
		createdBy, err := auth.NewActorID(createdByValue)
		if err != nil {
			return nil, fmt.Errorf("rebuild specification creator: %w", err)
		}
		specification, err := runs.NewSpecification(id, revision, workflow, configuration, createdBy, createdAt)
		if err != nil {
			return nil, fmt.Errorf("rebuild specification: %w", err)
		}
		specifications = append(specifications, specification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read specifications: %w", err)
	}
	return specifications, nil
}

type queryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertSpecification(ctx context.Context, db queryer, proposalID runs.ProposalID, specification runs.Specification) error {
	_, err := db.ExecContext(ctx, `
        insert into run_specifications (
            id, proposal_id, revision, workflow_name, workflow_revision,
            normalization_version, normalized_document, created_by, created_at
        ) values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `,
		specification.ID().String(),
		proposalID.String(),
		specification.Revision(),
		specification.Workflow().Name(),
		specification.Workflow().Revision(),
		specification.Configuration().SchemaVersion(),
		specification.Configuration().Document(),
		specification.CreatedBy().String(),
		specification.CreatedAt(),
	)
	return classify("create specification", err)
}

func classify(operation string, err error) error {
	if err == nil {
		return nil
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return fmt.Errorf("%s: %w", operation, ErrConflict)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
