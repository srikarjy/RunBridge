package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/srikarjy/RunBridge/internal/auth"
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
