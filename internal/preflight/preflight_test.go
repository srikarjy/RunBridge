package preflight_test

import (
	"context"
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/authorization"
	"github.com/srikarjy/RunBridge/internal/preflight"
	"github.com/srikarjy/RunBridge/internal/projects"
	"github.com/srikarjy/RunBridge/internal/runs"
	"github.com/srikarjy/RunBridge/internal/runs/rnaseq"
)

type authorizer struct{ err error }

func (checker authorizer) Authorize(context.Context, auth.Actor, projects.ProjectID, authorization.Permission) error {
	return checker.err
}

func TestEvaluateReturnsReadyWhenAllChecksPass(t *testing.T) {
	specification, actor, projectID := validSpecification(t)
	result := preflight.Evaluate(context.Background(), specification, projectID, actor, authorizer{}, preflight.Limits{MaxSamples: 10, MaxCPUs: 16, MaxMemoryMiB: 32768})
	if !result.Ready() {
		t.Fatalf("result is not ready: %#v", result.BlockingFailures())
	}
	if len(result.Checks) != 7 {
		t.Fatalf("check count = %d, want 7", len(result.Checks))
	}
}

func TestEvaluateReportsAllResourceAndAuthorizationFailures(t *testing.T) {
	specification, actor, projectID := validSpecification(t)
	result := preflight.Evaluate(context.Background(), specification, projectID, actor, authorizer{err: authorization.ErrPermissionDenied}, preflight.Limits{MaxSamples: 0, MaxCPUs: 2, MaxMemoryMiB: 1024})
	if result.Ready() {
		t.Fatal("expected blocking failures")
	}
	wantCodes := []string{"resources.cpus.exceeded", "resources.memory.exceeded", "inputs.sample_limit.configured", "project.authorization"}
	for _, code := range wantCodes {
		if !hasCode(result, code) {
			t.Errorf("missing check code %q", code)
		}
	}
}

func TestEvaluateRejectsNonCanonicalConfiguration(t *testing.T) {
	actorID, _ := auth.NewActorID("actor-1")
	actor, _ := auth.NewActor(actorID, "Researcher", auth.ActorKindHuman)
	projectID, _ := projects.NewProjectID("project-1")
	workflow, _ := runs.NewWorkflowIdentifier(rnaseq.WorkflowName, "3.15.1")
	configuration, _ := runs.NewNormalizedConfiguration(rnaseq.NormalizationVersion, []byte(`{"resources":{"memory_mib":8192,"cpus":4},"profile":"docker","samples":[{"id":"sample-a","read1":"a_1","read2":"a_2"}],"reference":{"genome":"GRCh38","fasta":"ref.fa"},"parameters":{}}`))
	specificationID, _ := runs.NewSpecificationID("spec-1")
	specification, err := runs.NewSpecification(specificationID, 1, workflow, configuration, actorID, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	result := preflight.Evaluate(context.Background(), specification, projectID, actor, authorizer{}, preflight.Limits{MaxSamples: 10, MaxCPUs: 16, MaxMemoryMiB: 32768})
	if result.Ready() || !hasCode(result, "configuration.invalid") {
		t.Fatalf("expected canonical configuration failure: %#v", result)
	}
}

func hasCode(result preflight.Result, code string) bool {
	for _, check := range result.Checks {
		if check.Code == code {
			return true
		}
	}
	return false
}

func validSpecification(t *testing.T) (runs.Specification, auth.Actor, projects.ProjectID) {
	t.Helper()
	actorID, _ := auth.NewActorID("actor-1")
	actor, _ := auth.NewActor(actorID, "Researcher", auth.ActorKindHuman)
	projectID, _ := projects.NewProjectID("project-1")
	workflow, configuration, err := rnaseq.Normalize(rnaseq.Request{
		WorkflowRevision: "3.15.1",
		Samples:          []rnaseq.SampleInput{{ID: "sample-a", Read1: "a_1", Read2: "a_2"}},
		Reference:        rnaseq.ReferenceInput{Genome: "GRCh38", Fasta: "ref.fa"},
		Profile:          "docker",
		Resources:        rnaseq.ResourceRequest{CPUs: 4, MemoryMiB: 8192},
	})
	if err != nil {
		t.Fatal(err)
	}
	specificationID, _ := runs.NewSpecificationID("spec-1")
	specification, err := runs.NewSpecification(specificationID, 1, workflow, configuration, actorID, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	return specification, actor, projectID
}
