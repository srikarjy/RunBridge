# ADR-0006: Semantic Run Diff for rnaseq specifications

**Status:** Accepted

**Date:** 2026-09-09

**Decider:** Project maintainer

## Context

RunBridge must show meaningful execution changes before approval. Comparing serialized JSON text would make formatting differences noisy and would not explain which samples, references, resources, or workflow settings changed.

## Decision

The `rundiff` package compares two validated, canonical `nf-core/rnaseq` specifications. It emits machine-readable changes with a category, field path, kind, and before/after values. Samples are matched by stable sample ID; parameters and reference fields are compared by name; CPU and memory are compared as typed resource values; profile is treated as execution environment; workflow revision is compared separately.

Results carry both specification identities and sort changes by category and path for deterministic rendering. A missing baseline is explicit through `CompareOptional` and does not pretend that a first run has no review context. Unsupported workflows and malformed/non-canonical configurations fail closed.

## Consequences

- Reviewers can see scientific and resource-relevant changes before policy or approval.
- The diff remains deterministic and machine-readable without an LLM.
- Sample read path changes are distinct from sample additions/removals.
- The first implementation is intentionally rnaseq-specific; generalization waits for a second real workflow.
- Baseline access authorization remains the caller's responsibility and must use the project authorization boundary.
- Values may contain sensitive input locations; future API/audit layers must apply project access and redaction rules.
