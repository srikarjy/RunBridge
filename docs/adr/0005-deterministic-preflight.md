# ADR-0005: Deterministic preflight results

**Status:** Accepted

**Date:** 2026-09-09

**Decider:** Project maintainer

## Context

Before a run can reach policy or approval, RunBridge needs repeatable checks that explain structural problems, project access, and configured resource boundaries. These checks must evaluate the immutable normalized specification that will later be approved, rather than a mutable request or a formatted JSON string.

## Decision

The `preflight` package evaluates a persisted `runs.Specification` and returns a list of machine-readable check results with a code, status, field, and message. A result is ready only when it contains no failures. The evaluator reports all applicable failures in one pass.

Checks currently cover supported workflow name, workflow revision presence, canonical rnaseq configuration shape/version, project permission to propose a run, and caller-supplied maximum sample/CPU/memory limits. A missing or non-positive limit is a blocking configuration failure; no resource ceiling is invented in code.

The rnaseq package defensively validates persisted canonical bytes, rejects unknown fields and non-canonical ordering, and returns a small summary for preflight. It does not check remote object existence, dataset contents, scientific suitability, or Seqera availability.

## Consequences

- API and policy layers can render stable check codes without parsing prose.
- Reviewers see multiple corrections at once instead of a first-error loop.
- Persisted data is checked again at the preflight boundary, protecting against malformed or stale revisions.
- Resource limits must be provided by a trusted project configuration; Phase 17 may expand policy controls.
- Warnings are represented in the contract, while current checks are blocking or passing only.
- Preflight does not replace Run Diff, policy evaluation, approval, or external integration checks.
