# ADR 0013: Carry Request and Run Correlation IDs in Context

## Status

Accepted for Phase 14 foundation.

## Decision

Future handlers and workers carry request and run IDs through `context.Context`. The observability package generates opaque random IDs and enriches Go's `slog` logger with `request_id` and `run_id` fields. It has no dependency on a logging vendor.

## Consequences

Logs from a proposal, execution, webhook, or reconciliation attempt can be connected without exposing scientific inputs or credentials. Metrics and tracing can use the same identifiers when those systems are introduced.

