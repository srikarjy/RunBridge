package rnaseq_test

import (
	"errors"
	"testing"

	"github.com/srikarjy/RunBridge/internal/runs/rnaseq"
)

func TestNormalizeProducesStableSemanticBytes(t *testing.T) {
	first := rnaseq.Request{
		WorkflowRevision: " 3.15.1 ",
		Samples: []rnaseq.SampleInput{
			{ID: " sample-b ", Read1: " s3://reads/b_1.fastq.gz ", Read2: "s3://reads/b_2.fastq.gz"},
			{ID: "sample-a", Read1: "s3://reads/a_1.fastq.gz", Read2: "s3://reads/a_2.fastq.gz"},
		},
		Reference:  rnaseq.ReferenceInput{Genome: " GRCh38 ", Fasta: " s3://refs/GRCh38.fa ", GTF: "s3://refs/GRCh38.gtf"},
		Parameters: map[string]string{"  aligner ": "star", "trimmed": "true"},
		Profile:    " docker ",
		Resources:  rnaseq.ResourceRequest{CPUs: 8, MemoryMiB: 16384},
	}
	second := rnaseq.Request{
		WorkflowRevision: "3.15.1",
		Samples: []rnaseq.SampleInput{
			{ID: "sample-a", Read1: "s3://reads/a_1.fastq.gz", Read2: "s3://reads/a_2.fastq.gz"},
			{ID: "sample-b", Read1: "s3://reads/b_1.fastq.gz", Read2: "s3://reads/b_2.fastq.gz"},
		},
		Reference:  rnaseq.ReferenceInput{Genome: "GRCh38", Fasta: "s3://refs/GRCh38.fa", GTF: "s3://refs/GRCh38.gtf"},
		Parameters: map[string]string{"trimmed": "true", "aligner": "star"},
		Profile:    "docker",
		Resources:  rnaseq.ResourceRequest{CPUs: 8, MemoryMiB: 16384},
	}
	firstWorkflow, firstConfig, err := rnaseq.Normalize(first)
	if err != nil {
		t.Fatal(err)
	}
	secondWorkflow, secondConfig, err := rnaseq.Normalize(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstWorkflow != secondWorkflow {
		t.Fatalf("workflow identifiers differ: %#v vs %#v", firstWorkflow, secondWorkflow)
	}
	if string(firstConfig.Document()) != string(secondConfig.Document()) {
		t.Fatalf("equivalent requests produced different bytes:\n%s\n%s", firstConfig.Document(), secondConfig.Document())
	}
}

func TestNormalizeRejectsStructuralConflicts(t *testing.T) {
	base := validRequest()
	tests := []struct {
		name   string
		mutate func(*rnaseq.Request)
		want   error
	}{
		{"duplicate sample", func(request *rnaseq.Request) { request.Samples = append(request.Samples, request.Samples[0]) }, rnaseq.ErrDuplicateSampleID},
		{"missing read 2", func(request *rnaseq.Request) { request.Samples[0].Read2 = "" }, rnaseq.ErrRead2Required},
		{"missing reference input", func(request *rnaseq.Request) { request.Reference.Fasta = "" }, rnaseq.ErrReferenceInputRequired},
		{"zero CPUs", func(request *rnaseq.Request) { request.Resources.CPUs = 0 }, rnaseq.ErrCPUsInvalid},
		{"zero memory", func(request *rnaseq.Request) { request.Resources.MemoryMiB = 0 }, rnaseq.ErrMemoryInvalid},
		{"duplicate parameter after trimming", func(request *rnaseq.Request) {
			request.Parameters = map[string]string{"aligner": "star", " aligner ": "hisat2"}
		}, rnaseq.ErrDuplicateParameterName},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			request := base
			request.Samples = append([]rnaseq.SampleInput(nil), base.Samples...)
			testCase.mutate(&request)
			_, _, err := rnaseq.Normalize(request)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestNormalizeUsesOnlyNfCoreRnaseq(t *testing.T) {
	workflow, configuration, err := rnaseq.Normalize(validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Name() != rnaseq.WorkflowName {
		t.Fatalf("workflow name = %q", workflow.Name())
	}
	if configuration.SchemaVersion() != rnaseq.NormalizationVersion {
		t.Fatalf("schema version = %q", configuration.SchemaVersion())
	}
}

func validRequest() rnaseq.Request {
	return rnaseq.Request{
		WorkflowRevision: "3.14.0",
		Samples:          []rnaseq.SampleInput{{ID: "sample-a", Read1: "a_1.fastq.gz", Read2: "a_2.fastq.gz"}},
		Reference:        rnaseq.ReferenceInput{Genome: "GRCh38", Fasta: "GRCh38.fa"},
		Profile:          "docker",
		Resources:        rnaseq.ResourceRequest{CPUs: 4, MemoryMiB: 8192},
	}
}
