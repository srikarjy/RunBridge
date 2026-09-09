# ADR 0009: Validate Execution Transitions Before Persistence

## Status

Accepted for Phase 9 foundation.

## Decision

The execution domain owns a small explicit transition graph for `APPROVED`, `SUBMITTING`, `SUBMISSION_UNKNOWN`, `RUNNING`, and terminal outcomes. Same-state observations are idempotent. Terminal states cannot move again, and unknown Seqera status strings are errors rather than guessed mappings.

The eventual coordinator must persist transitions with an expected current state/version and write the corresponding audit event in the same database transaction. The transition package does not perform SQL or call Seqera.

## Rationale

An explicit graph makes invalid lifecycle changes visible before they reach PostgreSQL. It also keeps external observations conservative: queued or running remote work is treated as an identified nonterminal execution, while ambiguity remains represented by `SUBMISSION_UNKNOWN` until reconciliation.

## Consequences

This is a foundation, not the durable coordinator. Conditional updates, attempt records, idempotency keys, crash recovery, retry classification, cancellation races, and reconciliation are still required before live submissions are safe.

