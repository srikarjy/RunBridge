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
	"github.com/srikarjy/RunBridge/internal/audit"
	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/events"
	"github.com/srikarjy/RunBridge/internal/execution"
	"github.com/srikarjy/RunBridge/internal/integrity"
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

type AuditRecord struct {
	SequenceID       int64
	ID               string
	ProjectID        string
	ProposalID       *string
	ActorID          *string
	ActorKind        string
	EventType        string
	ObjectType       *string
	ObjectID         *string
	CorrelationID    *string
	SourceSystem     *string
	SourceEventID    *string
	SourceOccurredAt *time.Time
	RecordedAt       time.Time
	Metadata         json.RawMessage
}

type ExecutionRecord struct {
	ID                  string
	ProjectID           string
	ProposalID          string
	SpecificationID     string
	ApprovalID          string
	Status              runs.Status
	ExternalWorkspaceID *string
	ExternalExecutionID *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
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

// ClaimSubmission atomically creates the first pending attempt and claims the
// approved execution for submission. This is the transaction boundary that
// prevents concurrent workers from launching the same intent twice.
func (store *Store) ClaimSubmission(ctx context.Context, executionID, attemptID, correlationID string, number int64, startedAt time.Time) error {
	if strings.TrimSpace(executionID) == "" || strings.TrimSpace(attemptID) == "" || strings.TrimSpace(correlationID) == "" || number <= 0 || startedAt.IsZero() {
		return fmt.Errorf("claim submission: invalid attempt: %w", ErrConflict)
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("claim submission: begin transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `insert into execution_attempts (id, execution_id, attempt_number, correlation_id, status, started_at) values ($1, $2, $3, $4, 'pending', $5)`, attemptID, executionID, number, correlationID, startedAt); err != nil {
		return classify("claim submission attempt", err)
	}
	result, err := tx.ExecContext(ctx, `update executions set status = 'SUBMITTING', updated_at = $1 where id = $2 and status = 'APPROVED'`, startedAt, executionID)
	if err != nil {
		return classify("claim submission execution", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("claim submission: rows affected: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("claim submission: execution is not APPROVED: %w", ErrConflict)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("claim submission: commit: %w", err)
	}
	return nil
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

// RecordExternalEvent stores a webhook/poll observation before it is applied
// to execution state. The source/event pair is unique in PostgreSQL, making
// redelivery safe across process restarts.
func (store *Store) RecordExternalEvent(ctx context.Context, event events.Event, projectID string, proposalID *string, metadata any, recordedAt time.Time) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(projectID) == "" || recordedAt.IsZero() {
		return fmt.Errorf("record external event: project and timestamp are required: %w", ErrConflict)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("record external event: marshal metadata: %w", err)
	}
	var proposal any
	if proposalID != nil {
		proposal = *proposalID
	}
	_, err = store.db.ExecContext(ctx, `
        insert into audit_events (
            id, schema_version, project_id, proposal_id, actor_kind, event_type,
            object_type, object_id, source_system, source_event_id,
            source_occurred_at, recorded_at, metadata
        ) values ($1, 1, $2, $3, 'external', 'execution.status',
                  'external_execution', $4, $5, $6, $7, $8, $9::jsonb)
    `, event.ID, projectID, proposal, event.ExternalExecutionID, event.Source, event.ID, event.OccurredAt, recordedAt, string(metadataBytes))
	return classify("record external event", err)
}

// RecordAuditEvent appends an immutable domain event. The database sequence
// provides chronological ordering; callers never update an existing record.
func (store *Store) RecordAuditEvent(ctx context.Context, event audit.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	metadata, err := json.Marshal(event.CloneMetadata())
	if err != nil {
		return fmt.Errorf("record audit event: marshal metadata: %w", err)
	}
	var proposal, actor, objectType, objectID, correlation, source, sourceID any
	if event.ProposalID != "" {
		proposal = event.ProposalID
	}
	if event.ActorID != "" {
		actor = event.ActorID
	}
	if event.ObjectType != "" {
		objectType = event.ObjectType
	}
	if event.ObjectID != "" {
		objectID = event.ObjectID
	}
	if event.CorrelationID != "" {
		correlation = event.CorrelationID
	}
	if event.SourceSystem != "" {
		source, sourceID = event.SourceSystem, event.SourceEventID
	}
	_, err = store.db.ExecContext(ctx, `
        insert into audit_events (id, schema_version, project_id, proposal_id, actor_id, actor_kind,
            event_type, object_type, object_id, correlation_id, source_system, source_event_id,
            source_occurred_at, recorded_at, metadata)
        values ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb)
    `, event.ID, event.ProjectID, proposal, actor, event.ActorKind, event.Type, objectType, objectID, correlation, source, sourceID, event.SourceOccurredAt, event.RecordedAt, string(metadata))
	return classify("record audit event", err)
}

// ListAuditEvents returns the append-only timeline for one project. The
// project predicate is mandatory and the limit is bounded to keep a future
// API endpoint from turning an audit query into an unbounded scan.
func (store *Store) ListAuditEvents(ctx context.Context, projectID string, limit int) ([]AuditRecord, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("list audit events: project is required: %w", ErrConflict)
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := store.db.QueryContext(ctx, `
        select sequence_id, id, project_id, proposal_id, actor_id, actor_kind,
               event_type, object_type, object_id, correlation_id,
               source_system, source_event_id, source_occurred_at, recorded_at, metadata
        from audit_events where project_id = $1 order by sequence_id asc limit $2
    `, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	var records []AuditRecord
	for rows.Next() {
		var record AuditRecord
		if err := rows.Scan(&record.SequenceID, &record.ID, &record.ProjectID, &record.ProposalID, &record.ActorID, &record.ActorKind, &record.EventType, &record.ObjectType, &record.ObjectID, &record.CorrelationID, &record.SourceSystem, &record.SourceEventID, &record.SourceOccurredAt, &record.RecordedAt, &record.Metadata); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		record.Metadata = append(json.RawMessage(nil), record.Metadata...)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read audit events: %w", err)
	}
	return records, nil
}

// ApplyExternalEvent atomically appends an external event and advances its
// matching execution. The caller supplies the observed local state so the
// conditional update protects against concurrent workers.
func (store *Store) ApplyExternalEvent(ctx context.Context, event events.Event, projectID, executionID string, current runs.Status, currentOccurredAt time.Time, recordedAt time.Time) (events.Decision, error) {
	decision, err := events.Transition(current, currentOccurredAt, event)
	if err != nil || decision != events.Apply {
		return decision, err
	}
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(executionID) == "" {
		return events.Conflict, fmt.Errorf("apply external event: project and execution are required: %w", ErrConflict)
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return events.Conflict, fmt.Errorf("apply external event: begin transaction: %w", err)
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(ctx, `select exists(select 1 from audit_events where source_system = $1 and source_event_id = $2)`, event.Source, event.ID).Scan(&exists); err != nil {
		return events.Conflict, fmt.Errorf("apply external event: check duplicate: %w", err)
	}
	if exists {
		return events.Duplicate, nil
	}
	metadata := []byte(`{}`)
	_, err = tx.ExecContext(ctx, `
        insert into audit_events (id, schema_version, project_id, actor_kind, event_type,
            object_type, object_id, source_system, source_event_id, source_occurred_at,
            recorded_at, metadata)
        values ($1, 1, $2, 'external', 'execution.state_changed', 'execution', $3, $4, $5, $6, $7, $8::jsonb)
    `, event.ID, projectID, executionID, event.Source, event.ID, event.OccurredAt, recordedAt, string(metadata))
	if err != nil {
		return events.Conflict, classify("apply external event: record", err)
	}
	result, err := tx.ExecContext(ctx, `
        update executions set status = $1, external_execution_id = $2, updated_at = $3
        where id = $4 and project_id = $5 and status = $6
    `, event.Status, event.ExternalExecutionID, recordedAt, executionID, projectID, current)
	if err != nil {
		return events.Conflict, classify("apply external event: transition", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return events.Conflict, fmt.Errorf("apply external event: rows affected: %w", err)
	}
	if rows != 1 {
		return events.Conflict, fmt.Errorf("apply external event: stale execution state: %w", ErrConflict)
	}
	if err := tx.Commit(); err != nil {
		return events.Conflict, fmt.Errorf("apply external event: commit: %w", err)
	}
	return events.Apply, nil
}

// ListRecoverableExecutions returns nonterminal work that must be resumed or
// reconciled after a process restart.
func (store *Store) ListRecoverableExecutions(ctx context.Context, limit int) ([]ExecutionRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := store.db.QueryContext(ctx, `
        select id, project_id, proposal_id, specification_id, approval_id, status,
               external_workspace_id, external_execution_id, created_at, updated_at
        from executions where status in ('SUBMITTING', 'SUBMISSION_UNKNOWN', 'RUNNING')
        order by updated_at asc limit $1
    `, limit)
	if err != nil {
		return nil, fmt.Errorf("list recoverable executions: %w", err)
	}
	defer rows.Close()
	var records []ExecutionRecord
	for rows.Next() {
		var record ExecutionRecord
		if err := rows.Scan(&record.ID, &record.ProjectID, &record.ProposalID, &record.SpecificationID, &record.ApprovalID, &record.Status, &record.ExternalWorkspaceID, &record.ExternalExecutionID, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan recoverable execution: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read recoverable executions: %w", err)
	}
	return records, nil
}

func (store *Store) GetExecution(ctx context.Context, projectID, executionID string) (ExecutionRecord, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(executionID) == "" {
		return ExecutionRecord{}, fmt.Errorf("get execution: project and execution are required: %w", ErrConflict)
	}
	var record ExecutionRecord
	err := store.db.QueryRowContext(ctx, `
        select id, project_id, proposal_id, specification_id, approval_id, status,
               external_workspace_id, external_execution_id, created_at, updated_at
        from executions where project_id = $1 and id = $2
    `, projectID, executionID).Scan(&record.ID, &record.ProjectID, &record.ProposalID, &record.SpecificationID, &record.ApprovalID, &record.Status, &record.ExternalWorkspaceID, &record.ExternalExecutionID, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ExecutionRecord{}, ErrNotFound
	}
	if err != nil {
		return ExecutionRecord{}, fmt.Errorf("get execution: %w", err)
	}
	return record, nil
}

func (store *Store) CreateApprovalReceipt(ctx context.Context, approvalID string, receipt integrity.ApprovalReceipt, createdAt time.Time) error {
	if err := receipt.Validate(); err != nil {
		return err
	}
	if receipt.ApprovalID != approvalID || createdAt.IsZero() {
		return fmt.Errorf("create approval receipt: identity mismatch: %w", ErrConflict)
	}
	_, err := store.db.ExecContext(ctx, `insert into approval_receipts (approval_id, specification_digest, receipt_digest, created_at) values ($1, $2, $3, $4)`, approvalID, receipt.SpecificationDigest, receipt.DigestHex(), createdAt)
	return classify("create approval receipt", err)
}

func (store *Store) CreateExecutionReceipt(ctx context.Context, receipt integrity.ExecutionReceipt, createdAt time.Time) error {
	if err := receipt.Validate(); err != nil {
		return err
	}
	if createdAt.IsZero() {
		return fmt.Errorf("create execution receipt: created time is required: %w", ErrConflict)
	}
	_, err := store.db.ExecContext(ctx, `insert into execution_receipts (execution_id, approval_receipt_digest, manifest_digest, receipt_digest, external_execution_id, completed_at, created_at) values ($1, $2, $3, $4, $5, $6, $7)`, receipt.ExecutionID, receipt.ApprovalReceiptDigest, receipt.ManifestDigest, receipt.DigestHex(), receipt.ExternalExecutionID, receipt.CompletedAt, createdAt)
	return classify("create execution receipt", err)
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
