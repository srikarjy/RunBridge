package rundiff_test

import (
	"errors"
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/rundiff"
	"github.com/srikarjy/RunBridge/internal/runs"
	"github.com/srikarjy/RunBridge/internal/runs/rnaseq"
)

func TestCompareReportsSemanticChangesInStableOrder(t *testing.T) {
	baseline := specification(t, "spec-1", rnaseq.Request{
		WorkflowRevision: "3.14.0",
		Samples:          []rnaseq.SampleInput{{ID: "sample-a", Read1: "a_1", Read2: "a_2"}},
		Reference:        rnaseq.ReferenceInput{Genome: "GRCh38", Fasta: "GRCh38.fa"},
		Parameters:       map[string]string{"aligner": "star"}, Profile: "docker",
		Resources: rnaseq.ResourceRequest{CPUs: 8, MemoryMiB: 16384},
	})
	proposed := specification(t, "spec-2", rnaseq.Request{
		WorkflowRevision: "3.15.1",
		Samples: []rnaseq.SampleInput{
			{ID: "sample-b", Read1: "b_1", Read2: "b_2"},
			{ID: "sample-a", Read1: "a_1_changed", Read2: "a_2"},
		},
		Reference:  rnaseq.ReferenceInput{Genome: "GRCh37", Fasta: "GRCh37.fa"},
		Parameters: map[string]string{"aligner": "hisat2", "trimmed": "true"}, Profile: "singularity",
		Resources: rnaseq.ResourceRequest{CPUs: 32, MemoryMiB: 65536},
	})
	result, err := rundiff.Compare(baseline, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed() || result.BaselineMissing {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.Changes) != 10 {
		t.Fatalf("change count = %d, want 10", len(result.Changes))
	}
	for index := 1; index < len(result.Changes); index++ {
		previous, current := result.Changes[index-1], result.Changes[index]
		if previous.Category > current.Category || (previous.Category == current.Category && previous.Path > current.Path) {
			t.Fatalf("changes are not ordered: %#v", result.Changes)
		}
	}
	if !hasChange(result, rundiff.CategoryRevision, "workflow.revision", rundiff.ChangeChanged) {
		t.Fatal("missing revision change")
	}
	if !hasChange(result, rundiff.CategorySamples, "samples[sample-b]", rundiff.ChangeAdded) {
		t.Fatal("missing added sample")
	}
	if !hasChange(result, rundiff.CategoryInputs, "samples[sample-a].read1", rundiff.ChangeChanged) {
		t.Fatal("missing changed read")
	}
	if !hasChange(result, rundiff.CategoryReferences, "reference.genome", rundiff.ChangeChanged) {
		t.Fatal("missing reference change")
	}
	if !hasChange(result, rundiff.CategoryParameters, "parameters.trimmed", rundiff.ChangeAdded) {
		t.Fatal("missing added parameter")
	}
	if !hasChange(result, rundiff.CategoryResources, "resources.cpus", rundiff.ChangeChanged) {
		t.Fatal("missing CPU change")
	}
	if !hasChange(result, rundiff.CategoryExecutionEnvironment, "profile", rundiff.ChangeChanged) {
		t.Fatal("missing profile change")
	}
}

func TestCompareUnchangedSpecificationsHaveNoChanges(t *testing.T) {
	request := validRequest()
	baseline := specification(t, "spec-1", request)
	proposed := specification(t, "spec-2", request)
	result, err := rundiff.Compare(baseline, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed() || len(result.Changes) != 0 {
		t.Fatalf("unexpected changes: %#v", result.Changes)
	}
}

func TestCompareOptionalMakesMissingBaselineExplicit(t *testing.T) {
	proposed := specification(t, "spec-1", validRequest())
	result, err := rundiff.CompareOptional(nil, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if !result.BaselineMissing || result.Changed() {
		t.Fatalf("unexpected first-run result: %#v", result)
	}
}

func TestCompareRejectsUnsupportedOrMalformedSpecifications(t *testing.T) {
	baseline := specification(t, "spec-1", validRequest())
	actorID, _ := auth.NewActorID("actor-1")
	workflow, _ := runs.NewWorkflowIdentifier("other/workflow", "1.0.0")
	configuration := baseline.Configuration()
	otherID, _ := runs.NewSpecificationID("spec-other")
	unsupported, err := runs.NewSpecification(otherID, 1, workflow, configuration, actorID, time.Unix(2, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rundiff.Compare(baseline, unsupported); !errors.Is(err, rundiff.ErrUnsupportedWorkflow) {
		t.Fatalf("unsupported error = %v", err)
	}
	malformedConfig, _ := runs.NewNormalizedConfiguration(rnaseq.NormalizationVersion, []byte(`{"profile":"docker"}`))
	malformed, err := runs.NewSpecification(otherID, 1, baseline.Workflow(), malformedConfig, actorID, time.Unix(2, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rundiff.Compare(baseline, malformed); !errors.Is(err, rundiff.ErrInvalidConfiguration) {
		t.Fatalf("malformed error = %v", err)
	}
}

func hasChange(result rundiff.Result, category rundiff.Category, path string, kind rundiff.ChangeKind) bool {
	for _, change := range result.Changes {
		if change.Category == category && change.Path == path && change.Kind == kind {
			return true
		}
	}
	return false
}

func specification(t *testing.T, idValue string, request rnaseq.Request) runs.Specification {
	t.Helper()
	workflow, configuration, err := rnaseq.Normalize(request)
	if err != nil {
		t.Fatal(err)
	}
	actorID, _ := auth.NewActorID("actor-1")
	id, _ := runs.NewSpecificationID(idValue)
	specification, err := runs.NewSpecification(id, 1, workflow, configuration, actorID, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	return specification
}

func validRequest() rnaseq.Request {
	return rnaseq.Request{WorkflowRevision: "3.14.0", Samples: []rnaseq.SampleInput{{ID: "sample-a", Read1: "a_1", Read2: "a_2"}}, Reference: rnaseq.ReferenceInput{Genome: "GRCh38", Fasta: "GRCh38.fa"}, Profile: "docker", Resources: rnaseq.ResourceRequest{CPUs: 8, MemoryMiB: 16384}}
}
