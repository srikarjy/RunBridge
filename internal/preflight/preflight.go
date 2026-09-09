// Package preflight evaluates deterministic readiness checks for a persisted
// run specification. It has no HTTP, database, or Seqera dependencies.
package preflight

import (
	"context"
	"errors"
	"fmt"

	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/authorization"
	"github.com/srikarjy/RunBridge/internal/projects"
	"github.com/srikarjy/RunBridge/internal/runs"
	"github.com/srikarjy/RunBridge/internal/runs/rnaseq"
)

type CheckStatus string

const (
	StatusPass CheckStatus = "pass"
	StatusFail CheckStatus = "fail"
	StatusWarn CheckStatus = "warning"
)

type CheckResult struct {
	Code    string
	Status  CheckStatus
	Field   string
	Message string
}

type Result struct {
	Checks []CheckResult
}

func (result Result) Ready() bool {
	for _, check := range result.Checks {
		if check.Status == StatusFail {
			return false
		}
	}
	return len(result.Checks) > 0
}

func (result Result) BlockingFailures() []CheckResult {
	failures := make([]CheckResult, 0)
	for _, check := range result.Checks {
		if check.Status == StatusFail {
			failures = append(failures, check)
		}
	}
	return failures
}

type Limits struct {
	MaxSamples   int
	MaxCPUs      int
	MaxMemoryMiB int64
}

type Authorizer interface {
	Authorize(context.Context, auth.Actor, projects.ProjectID, authorization.Permission) error
}

// Evaluate runs structural and authorization checks against one immutable
// specification. It returns all applicable results rather than stopping at
// the first failure so a reviewer can correct a proposal in one pass.
func Evaluate(ctx context.Context, specification runs.Specification, projectID projects.ProjectID, actor auth.Actor, authorizer Authorizer, limits Limits) Result {
	result := Result{Checks: make([]CheckResult, 0, 8)}
	workflow := specification.Workflow()
	if workflow.Name() == rnaseq.WorkflowName {
		result.Checks = append(result.Checks, pass("workflow.supported", "workflow.name", "nf-core/rnaseq is supported"))
	} else {
		result.Checks = append(result.Checks, fail("workflow.unsupported", "workflow.name", fmt.Sprintf("workflow %q is not supported", workflow.Name())))
	}
	if workflow.Revision() != "" {
		result.Checks = append(result.Checks, pass("workflow.revision.present", "workflow.revision", "workflow revision is specified"))
	} else {
		result.Checks = append(result.Checks, fail("workflow.revision.required", "workflow.revision", "workflow revision is required"))
	}

	summary, err := rnaseq.ValidateNormalizedConfiguration(specification.Configuration())
	if err != nil {
		result.Checks = append(result.Checks, fail("configuration.invalid", "configuration", err.Error()))
	} else {
		result.Checks = append(result.Checks, pass("configuration.canonical", "configuration", "normalized configuration is canonical and structurally valid"))
		result = appendResourceChecks(result, summary, limits)
		result = appendSampleChecks(result, summary, limits)
	}

	if authorizer == nil {
		result.Checks = append(result.Checks, fail("project.authorization.unavailable", "project", "project authorization checker is not configured"))
	} else if err := authorizer.Authorize(ctx, actor, projectID, authorization.PermissionRunPropose); err != nil {
		status := "project authorization failed"
		if errors.Is(err, authorization.ErrPermissionDenied) {
			status = "actor is not permitted to propose runs in this project"
		} else if errors.Is(err, authorization.ErrMembershipRequired) {
			status = "actor is not a member of this project"
		}
		result.Checks = append(result.Checks, fail("project.authorization", "project", status))
	} else {
		result.Checks = append(result.Checks, pass("project.authorization", "project", "actor may propose runs in this project"))
	}
	return result
}

func appendResourceChecks(result Result, summary rnaseq.Summary, limits Limits) Result {
	if limits.MaxCPUs <= 0 {
		result.Checks = append(result.Checks, fail("resources.cpu_limit.configured", "resources.cpus", "maximum CPU limit is not configured"))
	} else if summary.CPUs > limits.MaxCPUs {
		result.Checks = append(result.Checks, fail("resources.cpus.exceeded", "resources.cpus", fmt.Sprintf("requested %d CPUs exceeds maximum %d", summary.CPUs, limits.MaxCPUs)))
	} else {
		result.Checks = append(result.Checks, pass("resources.cpus.allowed", "resources.cpus", "requested CPUs are within the project boundary"))
	}
	if limits.MaxMemoryMiB <= 0 {
		result.Checks = append(result.Checks, fail("resources.memory_limit.configured", "resources.memory_mib", "maximum memory limit is not configured"))
	} else if summary.MemoryMiB > limits.MaxMemoryMiB {
		result.Checks = append(result.Checks, fail("resources.memory.exceeded", "resources.memory_mib", fmt.Sprintf("requested %d MiB exceeds maximum %d MiB", summary.MemoryMiB, limits.MaxMemoryMiB)))
	} else {
		result.Checks = append(result.Checks, pass("resources.memory.allowed", "resources.memory_mib", "requested memory is within the project boundary"))
	}
	return result
}

func appendSampleChecks(result Result, summary rnaseq.Summary, limits Limits) Result {
	if limits.MaxSamples <= 0 {
		result.Checks = append(result.Checks, fail("inputs.sample_limit.configured", "samples", "maximum sample limit is not configured"))
	} else if summary.SampleCount > limits.MaxSamples {
		result.Checks = append(result.Checks, fail("inputs.samples.exceeded", "samples", fmt.Sprintf("requested %d samples exceeds maximum %d", summary.SampleCount, limits.MaxSamples)))
	} else {
		result.Checks = append(result.Checks, pass("inputs.samples.allowed", "samples", fmt.Sprintf("%d samples are within the project boundary", summary.SampleCount)))
	}
	return result
}

func pass(code, field, message string) CheckResult {
	return CheckResult{Code: code, Status: StatusPass, Field: field, Message: message}
}

func fail(code, field, message string) CheckResult {
	return CheckResult{Code: code, Status: StatusFail, Field: field, Message: message}
}
