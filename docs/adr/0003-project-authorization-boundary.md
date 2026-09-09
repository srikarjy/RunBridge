# ADR-0003: Project-scoped authorization boundary

**Status:** Accepted

**Date:** 2026-09-09

**Decider:** Project maintainer

## Context

RunBridge must isolate projects before it exposes run proposals, approvals, executions, or audit history. Phase 3 needs authorization without selecting or implementing an identity provider, HTTP middleware, token format, or session store.

## Decision

Treat an authenticated human `auth.Actor` as the principal entering the domain boundary. The `authorization` package resolves that actor's membership for the requested project through a `MembershipReader`, then evaluates a code-defined role-to-permission matrix. Every authorization check supplies both actor and project; the caller never supplies the role.

The initial permissions are project read/manage, run read/propose/review/cancel, and audit read. Viewer, runner, reviewer, and admin roles are explicit. A missing membership is reported without distinguishing an absent project from an absent membership. Database failures remain errors rather than being mistaken for denials.

Credential authentication remains an adapter concern for the future API. Machine identities and delegated agents remain deferred. The same server-side authorization boundary must protect future HTTP, job, and machine callers.

## Consequences

- Project isolation is testable without a live identity provider.
- Role grants stay reviewable in code while the product has one workflow family.
- PostgreSQL membership resolution is authoritative for each decision; caching is not introduced prematurely.
- Future authentication middleware must construct the principal from verified credentials and must not bypass `Authorizer`.
- Role changes and permission expansion require explicit code review and tests.
- Phase 18 may add machine principals only through the same authorization path.
