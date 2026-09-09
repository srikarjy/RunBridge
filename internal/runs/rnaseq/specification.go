// Package rnaseq defines the first workflow-specific RunBridge specification.
// It deliberately supports nf-core/rnaseq only.
package rnaseq

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/srikarjy/RunBridge/internal/runs"
)

const (
	WorkflowName         = "nf-core/rnaseq"
	NormalizationVersion = "rnaseq/v1"
)

var (
	ErrWorkflowRevisionRequired            = errors.New("workflow revision is required")
	ErrSampleRequired                      = errors.New("at least one sample is required")
	ErrSampleIDRequired                    = errors.New("sample ID is required")
	ErrDuplicateSampleID                   = errors.New("sample IDs must be unique")
	ErrRead1Required                       = errors.New("read 1 input is required")
	ErrRead2Required                       = errors.New("read 2 input is required")
	ErrReferenceGenomeRequired             = errors.New("reference genome is required")
	ErrReferenceInputRequired              = errors.New("at least one reference input is required")
	ErrProfileRequired                     = errors.New("compute profile is required")
	ErrCPUsInvalid                         = errors.New("CPUs must be greater than zero")
	ErrMemoryInvalid                       = errors.New("memory must be greater than zero")
	ErrParameterNameInvalid                = errors.New("parameter name is invalid")
	ErrDuplicateParameterName              = errors.New("parameter names must be unique after normalization")
	ErrNormalizedConfigurationInvalid      = errors.New("normalized rnaseq configuration is invalid")
	ErrNormalizedConfigurationNonCanonical = errors.New("normalized rnaseq configuration is not canonical")
)

// Request is the supported human-facing shape before normalization. Memory is
// explicitly represented in MiB; this avoids ambiguous unit parsing in the
// canonical specification.
type Request struct {
	WorkflowRevision string
	Samples          []SampleInput
	Reference        ReferenceInput
	Parameters       map[string]string
	Profile          string
	Resources        ResourceRequest
}

type SampleInput struct {
	ID    string
	Read1 string
	Read2 string
}

type ReferenceInput struct {
	Genome      string
	Fasta       string
	GTF         string
	STARIndex   string
	HISAT2Index string
}

type ResourceRequest struct {
	CPUs      int
	MemoryMiB int64
}

type canonicalDocument struct {
	Profile    string             `json:"profile"`
	Samples    []canonicalSample  `json:"samples"`
	Reference  canonicalReference `json:"reference"`
	Parameters map[string]string  `json:"parameters"`
	Resources  canonicalResources `json:"resources"`
}

type canonicalSample struct {
	ID    string `json:"id"`
	Read1 string `json:"read1"`
	Read2 string `json:"read2"`
}

type canonicalReference struct {
	Genome      string `json:"genome"`
	Fasta       string `json:"fasta,omitempty"`
	GTF         string `json:"gtf,omitempty"`
	STARIndex   string `json:"star_index,omitempty"`
	HISAT2Index string `json:"hisat2_index,omitempty"`
}

type canonicalResources struct {
	CPUs      int   `json:"cpus"`
	MemoryMiB int64 `json:"memory_mib"`
}

// Normalize validates and canonicalizes the supported rnaseq request. It
// returns a versioned opaque configuration whose bytes are stable for
// semantically equivalent requests.
func Normalize(request Request) (runs.WorkflowIdentifier, runs.NormalizedConfiguration, error) {
	revision := strings.TrimSpace(request.WorkflowRevision)
	if revision == "" {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, ErrWorkflowRevisionRequired
	}
	if len(request.Samples) == 0 {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, ErrSampleRequired
	}
	profile := strings.TrimSpace(request.Profile)
	if profile == "" {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, ErrProfileRequired
	}
	if request.Resources.CPUs <= 0 {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, ErrCPUsInvalid
	}
	if request.Resources.MemoryMiB <= 0 {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, ErrMemoryInvalid
	}

	samples := make([]canonicalSample, 0, len(request.Samples))
	seen := make(map[string]struct{}, len(request.Samples))
	for _, sample := range request.Samples {
		id := strings.TrimSpace(sample.ID)
		if id == "" {
			return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, fmt.Errorf("sample: %w", ErrSampleIDRequired)
		}
		if _, exists := seen[id]; exists {
			return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, fmt.Errorf("%q: %w", id, ErrDuplicateSampleID)
		}
		seen[id] = struct{}{}
		read1 := strings.TrimSpace(sample.Read1)
		if read1 == "" {
			return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, fmt.Errorf("sample %q: %w", id, ErrRead1Required)
		}
		read2 := strings.TrimSpace(sample.Read2)
		if read2 == "" {
			return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, fmt.Errorf("sample %q: %w", id, ErrRead2Required)
		}
		samples = append(samples, canonicalSample{ID: id, Read1: read1, Read2: read2})
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].ID < samples[j].ID })

	reference, err := normalizeReference(request.Reference)
	if err != nil {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, err
	}
	parameters, err := normalizeParameters(request.Parameters)
	if err != nil {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, err
	}

	document, err := json.Marshal(canonicalDocument{
		Profile:    profile,
		Samples:    samples,
		Reference:  reference,
		Parameters: parameters,
		Resources:  canonicalResources{CPUs: request.Resources.CPUs, MemoryMiB: request.Resources.MemoryMiB},
	})
	if err != nil {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, fmt.Errorf("marshal normalized rnaseq specification: %w", err)
	}
	workflow, err := runs.NewWorkflowIdentifier(WorkflowName, revision)
	if err != nil {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, err
	}
	configuration, err := runs.NewNormalizedConfiguration(NormalizationVersion, document)
	if err != nil {
		return runs.WorkflowIdentifier{}, runs.NormalizedConfiguration{}, err
	}
	return workflow, configuration, nil
}

// Summary contains the fields preflight and policy need without exposing the
// canonical document representation as a mutable API.
type Summary struct {
	SampleCount int
	Profile     string
	Genome      string
	CPUs        int
	MemoryMiB   int64
}

// ValidateNormalizedConfiguration verifies a persisted configuration's
// version, shape, required fields, ordering, and canonical bytes. It is a
// defensive check for data read from persistence; it does not access inputs.
func ValidateNormalizedConfiguration(configuration runs.NormalizedConfiguration) (Summary, error) {
	if configuration.SchemaVersion() != NormalizationVersion {
		return Summary{}, fmt.Errorf("%w: unsupported normalization version %q", ErrNormalizedConfigurationInvalid, configuration.SchemaVersion())
	}
	var document canonicalDocument
	decoder := json.NewDecoder(strings.NewReader(string(configuration.Document())))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Summary{}, fmt.Errorf("%w: decode document: %v", ErrNormalizedConfigurationInvalid, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Summary{}, fmt.Errorf("%w: multiple JSON values", ErrNormalizedConfigurationInvalid)
		}
		return Summary{}, fmt.Errorf("%w: trailing data: %v", ErrNormalizedConfigurationInvalid, err)
	}
	if document.Profile == "" || len(document.Samples) == 0 || document.Reference.Genome == "" || document.Resources.CPUs <= 0 || document.Resources.MemoryMiB <= 0 {
		return Summary{}, fmt.Errorf("%w: required field is missing or invalid", ErrNormalizedConfigurationInvalid)
	}
	for index, sample := range document.Samples {
		if sample.ID == "" || sample.Read1 == "" || sample.Read2 == "" {
			return Summary{}, fmt.Errorf("%w: sample %d is incomplete", ErrNormalizedConfigurationInvalid, index)
		}
		if index > 0 && document.Samples[index-1].ID >= sample.ID {
			return Summary{}, fmt.Errorf("%w: samples are not strictly sorted by ID", ErrNormalizedConfigurationInvalid)
		}
	}
	if document.Reference.Fasta == "" && document.Reference.GTF == "" && document.Reference.STARIndex == "" && document.Reference.HISAT2Index == "" {
		return Summary{}, fmt.Errorf("%w: reference input is missing", ErrNormalizedConfigurationInvalid)
	}
	canonical, err := json.Marshal(document)
	if err != nil {
		return Summary{}, fmt.Errorf("%w: re-encode document: %v", ErrNormalizedConfigurationInvalid, err)
	}
	if string(canonical) != string(configuration.Document()) {
		return Summary{}, ErrNormalizedConfigurationNonCanonical
	}
	return Summary{SampleCount: len(document.Samples), Profile: document.Profile, Genome: document.Reference.Genome, CPUs: document.Resources.CPUs, MemoryMiB: document.Resources.MemoryMiB}, nil
}

func normalizeReference(reference ReferenceInput) (canonicalReference, error) {
	result := canonicalReference{
		Genome:      strings.TrimSpace(reference.Genome),
		Fasta:       strings.TrimSpace(reference.Fasta),
		GTF:         strings.TrimSpace(reference.GTF),
		STARIndex:   strings.TrimSpace(reference.STARIndex),
		HISAT2Index: strings.TrimSpace(reference.HISAT2Index),
	}
	if result.Genome == "" {
		return canonicalReference{}, ErrReferenceGenomeRequired
	}
	if result.Fasta == "" && result.GTF == "" && result.STARIndex == "" && result.HISAT2Index == "" {
		return canonicalReference{}, ErrReferenceInputRequired
	}
	return result, nil
}

func normalizeParameters(parameters map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(parameters))
	for key, value := range parameters {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, ErrParameterNameInvalid
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("%q: %w", key, ErrDuplicateParameterName)
		}
		// Parameter values are opaque strings at this phase. Preserve whitespace
		// because it may be meaningful to a workflow parameter.
		result[key] = value
	}
	return result, nil
}
