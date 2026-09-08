package runs_test

import (
	"errors"
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/projects"
	"github.com/srikarjy/RunBridge/internal/runs"
)

func TestNormalizedConfigurationOwnsItsDocument(t *testing.T) {
	document := []byte(`{"memory":"16 GB"}`)
	configuration, err := runs.NewNormalizedConfiguration("v1", document)
	if err != nil {
		t.Fatal(err)
	}
	document[2] = 'X'
	returned := configuration.Document()
	returned[2] = 'Y'
	if got := string(configuration.Document()); got != `{"memory":"16 GB"}` {
		t.Fatalf("configuration document mutated: %s", got)
	}
}

func TestProposalRequiresSequentialImmutableRevisions(t *testing.T) {
	proposal := newProposal(t)
	second := newSpecification(t, "spec-2", 2, `{"memory":"64 GB"}`)
	if err := proposal.AddSpecification(second); err != nil {
		t.Fatalf("add second revision: %v", err)
	}
	if got := proposal.LatestSpecification().Revision(); got != 2 {
		t.Fatalf("latest revision = %d, want 2", got)
	}
	fourth := newSpecification(t, "spec-4", 4, `{"memory":"128 GB"}`)
	if err := proposal.AddSpecification(fourth); !errors.Is(err, runs.ErrRevisionSequence) {
		t.Fatalf("expected ErrRevisionSequence, got %v", err)
	}
	revisions := proposal.Specifications()
	revisions[0] = second
	if got := proposal.Specifications()[0].Revision(); got != 1 {
		t.Fatalf("caller mutated proposal revisions: %d", got)
	}
}

func TestProposalStartsAsDraft(t *testing.T) {
	proposal := newProposal(t)
	if proposal.Status() != runs.StatusDraft {
		t.Fatalf("status = %q, want %q", proposal.Status(), runs.StatusDraft)
	}
}

func TestProposalRejectsDuplicateSpecificationIdentity(t *testing.T) {
	proposal := newProposal(t)
	duplicate := newSpecification(t, "spec-1", 2, `{"memory":"64 GB"}`)
	if err := proposal.AddSpecification(duplicate); !errors.Is(err, runs.ErrDuplicateSpecificationID) {
		t.Fatalf("expected ErrDuplicateSpecificationID, got %v", err)
	}
}

func TestProposalRequiresInitialSpecificationCreator(t *testing.T) {
	proposalID, _ := runs.NewProposalID("proposal-1")
	projectID, _ := projects.NewProjectID("project-1")
	creatorID, _ := auth.NewActorID("actor-1")
	otherID, _ := auth.NewActorID("actor-2")
	workflow, _ := runs.NewWorkflowIdentifier("nf-core/rnaseq", "3.14.0")
	configuration, _ := runs.NewNormalizedConfiguration("v1", []byte(`{}`))
	specificationID, _ := runs.NewSpecificationID("spec-1")
	initial, err := runs.NewSpecification(specificationID, 1, workflow, configuration, otherID, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	_, err = runs.NewProposal(proposalID, projectID, creatorID, time.Unix(1, 0).UTC(), initial)
	if !errors.Is(err, runs.ErrSpecificationCreatorMismatch) {
		t.Fatalf("expected ErrSpecificationCreatorMismatch, got %v", err)
	}
}

func newProposal(t *testing.T) runs.Proposal {
	t.Helper()
	proposalID, _ := runs.NewProposalID("proposal-1")
	projectID, _ := projects.NewProjectID("project-1")
	actorID, _ := auth.NewActorID("actor-1")
	initial := newSpecification(t, "spec-1", 1, `{"memory":"16 GB"}`)
	proposal, err := runs.NewProposal(proposalID, projectID, actorID, time.Unix(1, 0).UTC(), initial)
	if err != nil {
		t.Fatal(err)
	}
	return proposal
}

func newSpecification(t *testing.T, idValue string, revision uint64, document string) runs.Specification {
	t.Helper()
	id, _ := runs.NewSpecificationID(idValue)
	actorID, _ := auth.NewActorID("actor-1")
	workflow, _ := runs.NewWorkflowIdentifier("nf-core/rnaseq", "3.14.0")
	configuration, err := runs.NewNormalizedConfiguration("v1", []byte(document))
	if err != nil {
		t.Fatal(err)
	}
	specification, err := runs.NewSpecification(id, revision, workflow, configuration, actorID, time.Unix(int64(revision), 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	return specification
}
