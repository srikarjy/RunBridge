# ADR-0001: Core domain boundaries

**Status:** Accepted

**Date:** 2026-09-08

**Decider:** Project maintainer

## Context

Phase 1 needs domain entities and invariants without coupling them to the future HTTP API, PostgreSQL schema, or Seqera payloads. Workflow-specific normalization, authorization rules, and execution transitions belong to later phases.

## Decision

Use small Go packages under `internal/`: `auth` for human actor identity, `projects` for projects and memberships, and `runs` for proposals, immutable specification revisions, workflow identity, normalized configuration values, and status vocabulary.

Constructors reject invalid identity and required values. A proposal owns an ordered sequence of specifications. Specifications and normalized documents are immutable from callers' perspective. Phase 1 defines status names but no transition engine. Normalized configuration is opaque and versioned until the rnaseq schema and normalization rules are implemented.

## Options considered

### One shared domain package

This is initially simple, but makes project identity, membership, and run lifecycle concerns harder to distinguish as authorization and persistence arrive.

### Small capability packages

This adds a few imports while preserving clear ownership and avoiding infrastructure dependencies. It matches the planned service boundaries without implying separate services.

### Framework-style entities with persistence tags

This could reduce mapping code later, but would make the domain depend on unchosen storage and API representations.

## Consequences

- Domain tests run without external services or dependencies.
- Persistence and transport adapters will map to domain values explicitly.
- `NormalizedConfiguration` cannot yet validate scientific fields; Phase 4 replaces its opaque document boundary with rnaseq-specific semantics.
- Role names are represented now, while their grants remain a Phase 3 authorization decision.
- Status values are shared vocabulary only; Phase 9 owns legal execution transitions and recovery.
- Machine identities remain deferred to Phase 18.
