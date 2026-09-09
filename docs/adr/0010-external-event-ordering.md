# ADR 0010: Treat External Events as At-Least-Once and Potentially Late

## Status

Accepted for Phase 11 foundation.

## Decision

Every external execution event must carry a source, source event ID, external execution ID, status, and occurrence timestamp. The pair `(source, event ID)` is the deduplication identity. Event processing accepts newer observations, ignores exact repeats and stale observations, and surfaces contradictory terminal evidence for reconciliation.

## Consequences

The eventual webhook handler must authenticate and persist an event before acknowledging it. A durable delivery record and replayable processor are still required; an in-memory cache cannot provide correctness across restarts.

