package integrity

import (
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/runs"
)

func TestSpecificationDigestBindsExactIntent(t *testing.T) {
	actor, _ := auth.NewActorID("actor-1")
	id, _ := runs.NewSpecificationID("spec-1")
	workflow, _ := runs.NewWorkflowIdentifier("nf-core/rnaseq", "3.18.0")
	config, _ := runs.NewNormalizedConfiguration("rnaseq/v1", []byte(`{"a":1}`))
	spec, _ := runs.NewSpecification(id, 1, workflow, config, actor, time.Unix(1, 0).UTC())
	first := DigestHex(spec)
	if len(first) != 64 {
		t.Fatalf("digest length: %d", len(first))
	}
	if first != DigestHex(spec) {
		t.Fatal("digest is not deterministic")
	}
	changed, _ := runs.NewNormalizedConfiguration("rnaseq/v1", []byte(`{"a":2}`))
	other, _ := runs.NewSpecification(id, 1, workflow, changed, actor, time.Unix(1, 0).UTC())
	if first == DigestHex(other) {
		t.Fatal("digest ignored exact bytes")
	}
}
