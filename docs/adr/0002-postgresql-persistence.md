# ADR-0002: PostgreSQL persistence foundation

**Status:** Accepted

**Date:** 2026-09-08

**Decider:** Project maintainer

## Context

RunBridge needs durable project, proposal, approval, execution, and audit state. A proposal and its specification revisions must survive process restarts, remain project-scoped, and support later concurrency controls. Future approval hashes must refer to the exact normalized bytes that were approved.

## Decision

Use PostgreSQL as the system of record and `pgx` through Go's `database/sql` interface. Keep ordered SQL migrations embedded in the service package. A transaction-scoped PostgreSQL advisory lock serializes migration application across application instances.

Use application-issued text identifiers at the domain boundary and PostgreSQL identity values only for local audit ordering. Store normalized specifications as `bytea` to retain exact canonical bytes. Store structured review and audit metadata as `jsonb`, where database normalization does not affect approval identity.

Enforce required relationships, controlled role/status vocabularies, proposal revision uniqueness, approval-to-specification relationships, and external execution identity uniqueness in the schema. Index foreign keys and expected project/status/timeline access paths.

Aggregate writes use short transactions. No transaction remains open across an external call. Store methods take `context.Context`, use parameterized queries, and return stable not-found/conflict errors while retaining the underlying database error.

## Options considered

### PostgreSQL with exact-byte specifications

This preserves approval inputs precisely, supports transactional constraints, and matches the reliability model. Configuration fields are not directly queryable until deliberate projections are added.

### PostgreSQL `jsonb` specifications

This supports field queries, but PostgreSQL may normalize object representation. It is unsuitable as the sole representation of bytes used for future integrity hashes.

### In-memory persistence first

This would simplify early demos but would avoid the restart, concurrency, and transaction properties central to RunBridge.

### ORM-generated schema and entities

This would reduce mapping code but couple domain types to a framework and obscure the constraints and transaction boundaries that matter to the project.

## Consequences

- PostgreSQL schema constraints provide durable structural integrity in addition to Go domain validation.
- Exact normalized bytes can later be hashed without database reserialization.
- Domain and persistence representations require explicit mapping.
- Schema migrations are part of the application artifact and run once under a database lock.
- Integration tests need a real PostgreSQL instance supplied through `RUNBRIDGE_TEST_DATABASE_URL`.
- Authorization decisions, lifecycle transition logic, and audit timeline APIs remain in their later phases.
