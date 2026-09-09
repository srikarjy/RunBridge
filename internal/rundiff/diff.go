// Package rundiff compares normalized run specifications semantically. It is
// deliberately workflow-specific to the first nf-core/rnaseq integration.
package rundiff

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/srikarjy/RunBridge/internal/runs"
	"github.com/srikarjy/RunBridge/internal/runs/rnaseq"
)

var (
	ErrUnsupportedWorkflow  = errors.New("run diff supports nf-core/rnaseq only")
	ErrInvalidConfiguration = errors.New("run diff configuration is invalid")
)

type Category string

const (
	CategoryWorkflow             Category = "workflow"
	CategoryRevision             Category = "revision"
	CategoryInputs               Category = "inputs"
	CategorySamples              Category = "samples"
	CategoryParameters           Category = "parameters"
	CategoryResources            Category = "resources"
	CategoryReferences           Category = "references"
	CategoryExecutionEnvironment Category = "execution_environment"
)

type ChangeKind string

const (
	ChangeAdded   ChangeKind = "added"
	ChangeRemoved ChangeKind = "removed"
	ChangeChanged ChangeKind = "changed"
)

type Change struct {
	Category Category   `json:"category"`
	Path     string     `json:"path"`
	Kind     ChangeKind `json:"kind"`
	Before   *string    `json:"before,omitempty"`
	After    *string    `json:"after,omitempty"`
}

type Result struct {
	BaselineSpecificationID runs.SpecificationID `json:"baseline_specification_id"`
	ProposedSpecificationID runs.SpecificationID `json:"proposed_specification_id"`
	BaselineMissing         bool                 `json:"baseline_missing"`
	Changes                 []Change             `json:"changes"`
}

func (result Result) Changed() bool { return len(result.Changes) > 0 }

// Compare produces a stable semantic diff between two normalized
// specifications. Both configurations are defensively validated before any
// fields are compared.
func Compare(baseline, proposed runs.Specification) (Result, error) {
	result := Result{BaselineSpecificationID: baseline.ID(), ProposedSpecificationID: proposed.ID(), Changes: make([]Change, 0)}
	if baseline.Workflow().Name() != rnaseq.WorkflowName || proposed.Workflow().Name() != rnaseq.WorkflowName {
		return Result{}, ErrUnsupportedWorkflow
	}
	if baseline.Workflow().Revision() != proposed.Workflow().Revision() {
		result.Changes = append(result.Changes, changed(CategoryRevision, "workflow.revision", baseline.Workflow().Revision(), proposed.Workflow().Revision()))
	}
	baselineDocument, err := decode(baseline)
	if err != nil {
		return Result{}, err
	}
	proposedDocument, err := decode(proposed)
	if err != nil {
		return Result{}, err
	}
	compareDocument(&result, baselineDocument, proposedDocument)
	sortChanges(result.Changes)
	return result, nil
}

// CompareOptional makes a missing baseline explicit for first-run proposals.
func CompareOptional(baseline *runs.Specification, proposed runs.Specification) (Result, error) {
	if baseline == nil {
		if proposed.Workflow().Name() != rnaseq.WorkflowName {
			return Result{}, ErrUnsupportedWorkflow
		}
		if _, err := decode(proposed); err != nil {
			return Result{}, err
		}
		return Result{ProposedSpecificationID: proposed.ID(), BaselineMissing: true, Changes: []Change{}}, nil
	}
	return Compare(*baseline, proposed)
}

type document struct {
	Profile    string            `json:"profile"`
	Samples    []sample          `json:"samples"`
	Reference  reference         `json:"reference"`
	Parameters map[string]string `json:"parameters"`
	Resources  resources         `json:"resources"`
}

type sample struct {
	ID    string `json:"id"`
	Read1 string `json:"read1"`
	Read2 string `json:"read2"`
}

type reference struct {
	Genome      string `json:"genome"`
	Fasta       string `json:"fasta,omitempty"`
	GTF         string `json:"gtf,omitempty"`
	STARIndex   string `json:"star_index,omitempty"`
	HISAT2Index string `json:"hisat2_index,omitempty"`
}

type resources struct {
	CPUs      int   `json:"cpus"`
	MemoryMiB int64 `json:"memory_mib"`
}

func decode(specification runs.Specification) (document, error) {
	if specification.Configuration().SchemaVersion() != rnaseq.NormalizationVersion {
		return document{}, fmt.Errorf("%w: unsupported normalization version %q", ErrInvalidConfiguration, specification.Configuration().SchemaVersion())
	}
	if _, err := rnaseq.ValidateNormalizedConfiguration(specification.Configuration()); err != nil {
		return document{}, fmt.Errorf("%w: %v", ErrInvalidConfiguration, err)
	}
	var result document
	if err := json.Unmarshal(specification.Configuration().Document(), &result); err != nil {
		return document{}, fmt.Errorf("%w: decode canonical document: %v", ErrInvalidConfiguration, err)
	}
	return result, nil
}

func compareDocument(result *Result, baseline, proposed document) {
	if baseline.Profile != proposed.Profile {
		result.Changes = append(result.Changes, changed(CategoryExecutionEnvironment, "profile", baseline.Profile, proposed.Profile))
	}
	compareSamples(result, baseline.Samples, proposed.Samples)
	compareReference(result, baseline.Reference, proposed.Reference)
	compareParameters(result, baseline.Parameters, proposed.Parameters)
	if baseline.Resources.CPUs != proposed.Resources.CPUs {
		result.Changes = append(result.Changes, changed(CategoryResources, "resources.cpus", fmt.Sprintf("%d", baseline.Resources.CPUs), fmt.Sprintf("%d", proposed.Resources.CPUs)))
	}
	if baseline.Resources.MemoryMiB != proposed.Resources.MemoryMiB {
		result.Changes = append(result.Changes, changed(CategoryResources, "resources.memory_mib", fmt.Sprintf("%d", baseline.Resources.MemoryMiB), fmt.Sprintf("%d", proposed.Resources.MemoryMiB)))
	}
}

func compareSamples(result *Result, baseline, proposed []sample) {
	baselineByID := make(map[string]sample, len(baseline))
	proposedByID := make(map[string]sample, len(proposed))
	for _, sample := range baseline {
		baselineByID[sample.ID] = sample
	}
	for _, sample := range proposed {
		proposedByID[sample.ID] = sample
	}
	ids := unionKeys(baselineByID, proposedByID)
	for _, id := range ids {
		before, beforeExists := baselineByID[id]
		after, afterExists := proposedByID[id]
		switch {
		case !beforeExists:
			result.Changes = append(result.Changes, added(CategorySamples, "samples["+id+"]", fmt.Sprintf("read1=%s; read2=%s", after.Read1, after.Read2)))
		case !afterExists:
			result.Changes = append(result.Changes, removed(CategorySamples, "samples["+id+"]", fmt.Sprintf("read1=%s; read2=%s", before.Read1, before.Read2)))
		default:
			if before.Read1 != after.Read1 {
				result.Changes = append(result.Changes, changed(CategoryInputs, "samples["+id+"].read1", before.Read1, after.Read1))
			}
			if before.Read2 != after.Read2 {
				result.Changes = append(result.Changes, changed(CategoryInputs, "samples["+id+"].read2", before.Read2, after.Read2))
			}
		}
	}
}

func compareReference(result *Result, baseline, proposed reference) {
	values := []struct{ path, before, after string }{
		{"reference.genome", baseline.Genome, proposed.Genome},
		{"reference.fasta", baseline.Fasta, proposed.Fasta},
		{"reference.gtf", baseline.GTF, proposed.GTF},
		{"reference.star_index", baseline.STARIndex, proposed.STARIndex},
		{"reference.hisat2_index", baseline.HISAT2Index, proposed.HISAT2Index},
	}
	for _, value := range values {
		if value.before != value.after {
			result.Changes = append(result.Changes, changed(CategoryReferences, value.path, value.before, value.after))
		}
	}
}

func compareParameters(result *Result, baseline, proposed map[string]string) {
	keys := unionKeys(baseline, proposed)
	for _, key := range keys {
		before, beforeExists := baseline[key]
		after, afterExists := proposed[key]
		switch {
		case !beforeExists:
			result.Changes = append(result.Changes, added(CategoryParameters, "parameters."+key, after))
		case !afterExists:
			result.Changes = append(result.Changes, removed(CategoryParameters, "parameters."+key, before))
		case before != after:
			result.Changes = append(result.Changes, changed(CategoryParameters, "parameters."+key, before, after))
		}
	}
}

func unionKeys[T any](first, second map[string]T) []string {
	keys := make(map[string]struct{}, len(first)+len(second))
	for key := range first {
		keys[key] = struct{}{}
	}
	for key := range second {
		keys[key] = struct{}{}
	}
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func sortChanges(changes []Change) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Category != changes[j].Category {
			return changes[i].Category < changes[j].Category
		}
		return changes[i].Path < changes[j].Path
	})
}

func value(value string) *string { return &value }
func changed(category Category, path, before, after string) Change {
	return Change{Category: category, Path: path, Kind: ChangeChanged, Before: value(before), After: value(after)}
}
func added(category Category, path, after string) Change {
	return Change{Category: category, Path: path, Kind: ChangeAdded, After: value(after)}
}
func removed(category Category, path, before string) Change {
	return Change{Category: category, Path: path, Kind: ChangeRemoved, Before: value(before)}
}
