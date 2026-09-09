# ADR-0004: Canonical nf-core/rnaseq specification

**Status:** Accepted

**Date:** 2026-09-09

**Decider:** Project maintainer

## Context

RunBridge's first workflow must be concrete enough for deterministic validation, Run Diff, approval, and later Seqera translation. Comparing caller-supplied JSON would preserve formatting differences and leave units, ordering, and defaults ambiguous.

## Decision

Support one workflow family in this phase: `nf-core/rnaseq`. The workflow revision is explicit. A request contains paired sample inputs, a reference genome plus at least one reference input, a named compute profile, typed CPU/memory resources, and opaque string parameters.

Normalize identifiers and paths by trimming surrounding whitespace, sort samples by unique sample ID, preserve parameter values exactly, trim parameter names, and encode the canonical document using a fixed struct shape with deterministic JSON map-key ordering. Memory is represented as positive MiB before normalization. The output is stored in the existing versioned `NormalizedConfiguration` as `rnaseq/v1` bytes and paired with the `nf-core/rnaseq` workflow identifier.

## Consequences

- Equivalent requests with different sample order or harmless surrounding whitespace produce identical configuration bytes.
- Sample IDs and parameter names cannot collide after normalization.
- Parameter values remain opaque strings; typed parameter semantics and workflow allowlists belong to later validation/policy work.
- The current reference check is structural and does not verify remote object existence, content, or scientific suitability.
- A future normalization version must be recorded so old approval hashes are never reinterpreted.
- Additional workflows require separate, explicit models rather than a generic workflow DSL.
