# ADR 0008: Keep Seqera Behind a Narrow Transport Boundary

## Status

Accepted for Phase 8.

## Decision

RunBridge uses a small Go adapter in `integrations/seqera` for the first Seqera integration. It targets the Platform API operations needed by the initial vertical slice:

- `POST /workflow/launch` with a workspace ID query parameter
- `GET /workflow/{workflowId}`
- `POST /workflow/{workflowId}/cancel`

The adapter sends bearer authentication and `Accept-Version: 1`, validates required launch identity fields, and returns external IDs/statuses. It does not persist state, decide authorization, retry unsafe submissions, or expose an HTTP server. Contract tests use a fake transport and never contact Seqera.

## Rationale

Seqera is an unreliable external boundary. Keeping transport concerns isolated lets the execution service own durable transitions, attempt records, idempotency, and reconciliation. The documented API supports launch, workflow inspection, and cancellation, while the safe correlation and duplicate-submission strategy still needs to be designed in later phases.

## Consequences

The adapter is intentionally incomplete for production execution. A later phase must define the exact projection from an approved canonical rnaseq specification, classify external statuses, record submission attempts, and reconcile `SUBMISSION_UNKNOWN` before retrying. No live token or fabricated run result belongs in this repository.

