# ADR-0007: Bind approval to immutable run intent

**Status:** Accepted

**Date:** 2026-09-09

**Decider:** Project maintainer

## Context

RunBridge must establish that a reviewer approved the specification that will eventually execute. A proposal can receive later revisions, and policy may decide that some unchanged runs do not need a human reviewer.

## Decision

Use deterministic `policy/v1` rules: a blocking preflight failure is denied; a first run or any non-empty Run Diff requires review; an unchanged run with passing preflight receives an explicit `policy_approved` decision. Human approvals are `approved` or `rejected` and require a reviewer identity.

An approval stores project, proposal, specification ID and revision, decision, policy version, decision time, and copied preflight/Run Diff context. It points to the selected immutable specification instead of the proposal's mutable latest pointer. An approval is current only while that specification remains the proposal's latest revision; older approvals remain historical evidence but cannot authorize a changed proposal.

The persistence adapter stores the decision and review context in PostgreSQL. It does not perform external calls or change proposal state. API/application orchestration will later enforce reviewer project authorization, policy freshness, and the transition to execution.

## Consequences

- Policy decisions are explainable and reproducible without a policy DSL or LLM.
- A first run always receives an explicit human-review requirement under the initial policy.
- No-review authorization remains auditable without inventing a reviewer.
- Approval evidence is defensively copied so later callers cannot mutate it through shared slices or pointers.
- Approval does not yet perform cryptographic hashing; Phase 13 adds integrity receipts.
- Reviewer separation-of-duties, revocation, freshness, and API authentication remain explicit follow-up decisions.
