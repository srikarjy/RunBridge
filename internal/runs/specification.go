// Package runs defines proposals and immutable run specification revisions.
package runs

import (
	"errors"
	"strings"
	"time"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/projects"
)

var (
	ErrProposalIDRequired           = errors.New("proposal ID is required")
	ErrSpecificationIDRequired      = errors.New("specification ID is required")
	ErrWorkflowNameRequired         = errors.New("workflow name is required")
	ErrWorkflowRevisionRequired     = errors.New("workflow revision is required")
	ErrNormalizationVersionRequired = errors.New("normalization version is required")
	ErrNormalizedDocumentRequired   = errors.New("normalized document is required")
	ErrRevisionSequence             = errors.New("specification revision must be the next revision")
	ErrCreatedAtRequired            = errors.New("creation time is required")
	ErrSpecificationCreatorMismatch = errors.New("initial specification creator must match proposal creator")
	ErrDuplicateSpecificationID     = errors.New("specification ID is already used by this proposal")
)

type ProposalID string

func NewProposalID(value string) (ProposalID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrProposalIDRequired
	}
	return ProposalID(value), nil
}

func (id ProposalID) String() string { return string(id) }

type SpecificationID string

func NewSpecificationID(value string) (SpecificationID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrSpecificationIDRequired
	}
	return SpecificationID(value), nil
}

func (id SpecificationID) String() string { return string(id) }

type WorkflowIdentifier struct {
	name     string
	revision string
}

func NewWorkflowIdentifier(name, revision string) (WorkflowIdentifier, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return WorkflowIdentifier{}, ErrWorkflowNameRequired
	}
	revision = strings.TrimSpace(revision)
	if revision == "" {
		return WorkflowIdentifier{}, ErrWorkflowRevisionRequired
	}
	return WorkflowIdentifier{name: name, revision: revision}, nil
}

func (workflow WorkflowIdentifier) Name() string     { return workflow.name }
func (workflow WorkflowIdentifier) Revision() string { return workflow.revision }

// NormalizedConfiguration is opaque in Phase 1. Phase 4 defines rnaseq
// normalization rules; this value establishes versioning and immutability now.
type NormalizedConfiguration struct {
	schemaVersion string
	document      []byte
}

func NewNormalizedConfiguration(schemaVersion string, document []byte) (NormalizedConfiguration, error) {
	schemaVersion = strings.TrimSpace(schemaVersion)
	if schemaVersion == "" {
		return NormalizedConfiguration{}, ErrNormalizationVersionRequired
	}
	if len(document) == 0 {
		return NormalizedConfiguration{}, ErrNormalizedDocumentRequired
	}
	return NormalizedConfiguration{schemaVersion: schemaVersion, document: cloneBytes(document)}, nil
}

func (c NormalizedConfiguration) SchemaVersion() string { return c.schemaVersion }
func (c NormalizedConfiguration) Document() []byte      { return cloneBytes(c.document) }

type Specification struct {
	id            SpecificationID
	revision      uint64
	workflow      WorkflowIdentifier
	configuration NormalizedConfiguration
	createdBy     auth.ActorID
	createdAt     time.Time
}

func NewSpecification(id SpecificationID, revision uint64, workflow WorkflowIdentifier, configuration NormalizedConfiguration, createdBy auth.ActorID, createdAt time.Time) (Specification, error) {
	if strings.TrimSpace(id.String()) == "" {
		return Specification{}, ErrSpecificationIDRequired
	}
	if revision == 0 {
		return Specification{}, ErrRevisionSequence
	}
	if strings.TrimSpace(workflow.Name()) == "" {
		return Specification{}, ErrWorkflowNameRequired
	}
	if strings.TrimSpace(workflow.Revision()) == "" {
		return Specification{}, ErrWorkflowRevisionRequired
	}
	if strings.TrimSpace(configuration.SchemaVersion()) == "" {
		return Specification{}, ErrNormalizationVersionRequired
	}
	if strings.TrimSpace(createdBy.String()) == "" {
		return Specification{}, auth.ErrActorIDRequired
	}
	if createdAt.IsZero() {
		return Specification{}, ErrCreatedAtRequired
	}
	return Specification{id: id, revision: revision, workflow: workflow, configuration: configuration, createdBy: createdBy, createdAt: createdAt}, nil
}

func (specification Specification) ID() SpecificationID          { return specification.id }
func (specification Specification) Revision() uint64             { return specification.revision }
func (specification Specification) Workflow() WorkflowIdentifier { return specification.workflow }
func (specification Specification) Configuration() NormalizedConfiguration {
	return specification.configuration
}
func (specification Specification) CreatedBy() auth.ActorID { return specification.createdBy }
func (specification Specification) CreatedAt() time.Time    { return specification.createdAt }

// Status is shared vocabulary for the future lifecycle state machine.
type Status string

const (
	StatusDraft             Status = "DRAFT"
	StatusPreflighted       Status = "PREFLIGHTED"
	StatusAwaitingApproval  Status = "AWAITING_APPROVAL"
	StatusApproved          Status = "APPROVED"
	StatusSubmitting        Status = "SUBMITTING"
	StatusSubmissionUnknown Status = "SUBMISSION_UNKNOWN"
	StatusRunning           Status = "RUNNING"
	StatusSucceeded         Status = "SUCCEEDED"
	StatusFailed            Status = "FAILED"
	StatusCancelled         Status = "CANCELLED"
	StatusRejected          Status = "REJECTED"
)

type Proposal struct {
	id        ProposalID
	projectID projects.ProjectID
	createdBy auth.ActorID
	createdAt time.Time
	status    Status
	revisions []Specification
}

func NewProposal(id ProposalID, projectID projects.ProjectID, createdBy auth.ActorID, createdAt time.Time, initial Specification) (Proposal, error) {
	if strings.TrimSpace(id.String()) == "" {
		return Proposal{}, ErrProposalIDRequired
	}
	if strings.TrimSpace(projectID.String()) == "" {
		return Proposal{}, projects.ErrProjectIDRequired
	}
	if strings.TrimSpace(createdBy.String()) == "" {
		return Proposal{}, auth.ErrActorIDRequired
	}
	if createdAt.IsZero() {
		return Proposal{}, ErrCreatedAtRequired
	}
	if initial.Revision() != 1 {
		return Proposal{}, ErrRevisionSequence
	}
	if initial.CreatedBy() != createdBy {
		return Proposal{}, ErrSpecificationCreatorMismatch
	}
	return Proposal{id: id, projectID: projectID, createdBy: createdBy, createdAt: createdAt, status: StatusDraft, revisions: []Specification{initial}}, nil
}

func (p Proposal) ID() ProposalID                { return p.id }
func (p Proposal) ProjectID() projects.ProjectID { return p.projectID }
func (p Proposal) CreatedBy() auth.ActorID       { return p.createdBy }
func (p Proposal) CreatedAt() time.Time          { return p.createdAt }
func (p Proposal) Status() Status                { return p.status }

func (p Proposal) LatestSpecification() Specification {
	return p.revisions[len(p.revisions)-1]
}

func (p Proposal) Specifications() []Specification {
	return append([]Specification(nil), p.revisions...)
}

// AddSpecification adds the next immutable draft revision. Later phases define
// which lifecycle states permit revision creation.
func (p *Proposal) AddSpecification(specification Specification) error {
	next := uint64(len(p.revisions) + 1)
	if specification.Revision() != next {
		return ErrRevisionSequence
	}
	for _, existing := range p.revisions {
		if existing.ID() == specification.ID() {
			return ErrDuplicateSpecificationID
		}
	}
	p.revisions = append(p.revisions, specification)
	return nil
}

func cloneBytes(value []byte) []byte { return append([]byte(nil), value...) }
