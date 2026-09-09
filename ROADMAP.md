# RunBridge roadmap

Phases are implementation gates, not dates. **Phases 0 through 8 are complete; foundations for Phases 9 through 15 are implemented, with their full completion gates still in progress.** The first product slice is a human-driven nf-core/rnaseq proposal through Seqera / Nextflow, with deterministic authorization and PostgreSQL-backed execution evidence.

The sequence builds capabilities incrementally; no live launch path should be exposed until authorization, durable execution, and ambiguous-submission handling are ready. Early Seqera integration work uses controlled adapters/fixtures, not an unguarded production launch endpoint. Audit persistence begins with domain mutations; Phase 12 completes timeline coverage and access. Cryptographic hardening later strengthens, rather than introduces, approval-to-execution correspondence.

## Phase 0 — Repository Foundation

**Complete: documentation only.** Establish repository structure, README, this roadmap, architecture, lifecycle, approval, audit, reliability design, configuration/deployment guidance, MIT License, and minimal dependency-free Go module metadata.

**Completion gate:** scope and invariants are documented; planned directories are tracked without source stubs; there is no runtime, database implementation, Seqera client, Run Diff logic, state machine, or cryptography.

## Phase 1 — Core Domain Model

**Complete.**

Define the minimum user/actor, project, membership, run proposal, run specification, workflow identifier, normalized configuration, and run status entities. Separate domain rules from HTTP, database details, and Seqera. Define revision identity, immutability, project ownership, and legal operation invariants without overgeneralizing workflow support.

**Completion gate:** clean domain tests demonstrate valid and invalid operations, especially revision and ownership invariants; domain packages do not depend on transport or external client types.

## Phase 2 — PostgreSQL Persistence

**Complete.**

Introduce migrations and persistence for projects, memberships, proposals, specification revisions, lifecycle state, execution metadata, and audit events. Add identity references and durable work records as required by the domain. Define transaction boundaries for state plus evidence, uniqueness for concurrent operations, and recovery-compatible updates. Avoid premature indexing and abstraction.

**Completion gate:** integration checks verify migrations, durable reads after restart, project-scoped references, rollback behavior, and concurrent mutation constraints.

## Phase 3 — Authentication + Project Authorization

**Complete for the domain authorization boundary.** HTTP credential authentication and identity-provider integration remain transport work for the future API.

Choose the minimum appropriate authentication mechanism. Enforce server-side project membership and permissions for commands and queries. Consider viewer, runner, reviewer, and admin roles; settle exact grants and reviewer separation rules during implementation. Avoid building a new identity platform.

**Completion gate:** tests reject unauthorized and cross-project access, including indirect specification/baseline/artifact references; identity and permissions are distinct concerns.

## Phase 4 — nf-core/rnaseq Run Specification

**Complete.**

Support exactly nf-core/rnaseq. Define workflow revision, samples and input identity, parameters, references, resource intent, and appropriate profile/compute context. Version normalization rules and handle defaults, units, unknown fields, missing values, and meaningful ordering explicitly. Pin or record mutable external dependencies as supported.

**Completion gate:** deterministic tests show semantically equivalent requests normalize consistently and meaningful changes remain distinguishable; no arbitrary-workflow execution is accepted.

## Phase 5 — Preflight Validation

**Complete.**

Implement structured deterministic checks for required inputs, sample metadata structure, supported revisions, resource limits, project eligibility, accessible inputs where verifiable, and obvious configuration conflicts. Distinguish blocking failures, warnings, and unavailable checks. Persist evidence tied to the specification and check version.

**Completion gate:** representative valid/invalid specifications yield stable field-specific results; failed required checks cannot advance to execution eligibility.

## Phase 6 — Run Diff

**Complete.**

Compare a selected accessible baseline, previous run, approved configuration, or another specification with the proposal. Produce structured changes categorized by workflow, revision, inputs, samples, parameters, resources, references, and execution environment. Bind results to both revision identities and normalization version. Represent the no-baseline condition explicitly.

**Completion gate:** machine-readable output and human-readable rendering agree; deterministic tests distinguish semantic changes from formatting/default equivalence and preserve order where meaningful. No LLM is involved in the core diff.

## Phase 7 — Approval Workflow

**Complete.**

Implement proposal → preflight/diff → deterministic approval requirement → reviewer decision → approved specification. Start with simple code/config rules. Record reviewer, timestamp, decision, reasons, immutable specification identity, policy version, and relevant diff/preflight context. Record no-review authorization explicitly when policy allows it.

**Completion gate:** stale review requests fail; execution-relevant changes require re-evaluation/re-approval; concurrent edits cannot redirect an approval to new intent. Define revocation, freshness, and submission-time authorization checks.

## Phase 8 — Seqera Integration

**Status: complete.** Add a narrow adapter for the documented Seqera Platform API. The boundary targets `POST /workflow/launch`, `GET /workflow/{workflowId}`, and `POST /workflow/{workflowId}/cancel`, sends bearer authentication and API version headers, and returns external identifiers/status without owning RunBridge lifecycle state. Contract tests use an in-memory transport; no live credentials or production launch path are included. Mapping the approved rnaseq specification into a verified launch payload, idempotency, retries, and reconciliation remain explicit follow-up work.

Build only the client operations required to submit rnaseq, retrieve state, resolve external IDs, and cancel where supported. Keep request translation and external error/state handling behind a narrow adapter. Verify actual launch schemas, authentication, idempotency/correlation options, pagination, rate limits, and cancellation semantics before relying on them.

**Completion gate:** contract/adapter tests verify approved-intent translation, sanitized errors, and supported operations. Document any inability to safely correlate uncertain launches. Do not expose live submissions before Phases 9–10 safeguards exist.

## Phase 9 — Durable Execution State Machine

**Status: in progress.** The execution package now validates the legal transition graph, protects terminal states, treats repeated observations as idempotent, and rejects unknown external statuses. PostgreSQL now persists executions with conditional transitions and records uniquely correlated submission attempts. The remaining work is transactional attempt claiming, retry classification, and restart recovery.

Persist legal transitions through APPROVED, SUBMITTING, RUNNING, terminal outcomes, and SUBMISSION_UNKNOWN, refining conceptual names as necessary. Introduce transactionally claimed attempts, idempotency, conditional updates, retry classification, and crash recovery. Keep cancellation intent distinct from confirmed cancellation.

**Completion gate:** concurrent commands cannot create duplicate local attempts; restarts recover unfinished work; state transitions and audit evidence commit together; ambiguous remote outcomes never trigger blind retries.

## Phase 10 — Submission Reconciliation

**Status: foundation complete.** The reconciliation package now makes conservative decisions for one external match, no match, multiple matches, and definitive failure. It does not query Seqera or change state; worker integration and persisted observation evidence remain.

Resolve SUBMITTING → network uncertainty → SUBMISSION_UNKNOWN using authoritative external observations. Correlate persisted attempts with remote executions. Handle no match, multiple matches, delayed visibility, and already completed runs. Only retry launches when evidence or verified external idempotency makes it safe; retain unresolved uncertainty otherwise.

**Completion gate:** lost responses and crashes after remote acceptance recover the existing execution without automatic duplication. Document operator resolution when the API cannot establish safe retry conditions.

## Phase 11 — Webhooks + Event Processing

**Status: foundation complete.** External events now have validated source/event identity, a durable deduplication key, conservative apply/duplicate/stale/conflict decisions, and a reusable HMAC verification primitive with replay protection. HTTP intake, vendor-specific authentication wiring, and replay workers remain.

Implement authenticated event intake, durable delivery records, idempotent consumption, duplicate handling, out-of-order awareness, and reconciliation. Acknowledge only after required persistence. Polling remains a recovery path; external events are not exactly-once.

**Completion gate:** duplicate/redelivered events do not duplicate transitions; late observations do not regress terminal states; conflicting observations trigger reconciliation; interrupted processing can replay safely.

## Phase 12 — Audit Timeline

**Status: foundation complete.** Audit events are append-only, external events are persisted with source identity, project-scoped timeline queries are available, and an authorization-aware HTTP handler is defined. Full event coverage and production authentication integration remain.

Complete append-oriented coverage and expose authorized API queries for proposal creation, preflight, diff, policy, approval request/decision, submission attempt, external ID assignment, state changes, completion, cancellation, and artifacts. Protect records against accidental mutation and define correction/retention procedures.

**Completion gate:** a run can be traced from request to approved revision to external execution and outcome; project isolation and redaction hold; state changes cannot silently lack audit evidence.

## Phase 13 — Integrity Hardening

**Status: foundation complete.** `internal/integrity` now derives deterministic SHA-256 digests for exact normalized specifications and canonical artifact manifests. Signing and receipt persistence remain future work.

Bind canonical approved specification → SHA-256 → approval receipt → execution evidence → artifact manifest → final execution receipt. Version canonicalization and receipt formats. Distinguish content identity from mutable resource locations. Consider chained audit hashes and AWS KMS signatures after basic verification works.

**Completion gate:** verification detects changed approved bytes and altered recorded artifact content when available; limits of external execution evidence are documented. Never claim signatures prove scientific correctness.

## Phase 14 — Observability

**Status: foundation complete.** `internal/observability` now provides request/run correlation IDs, standard-library structured logger enrichment, and atomic counters for key reliability signals. Tracing and production dashboards remain.

Expand structured logging, request/run/attempt correlation, error classification, integration/state metrics, and tracing where useful. Signals include submission failures, reconciliation attempts and unresolved age, duplicate webhook counts, transition conflicts, approval latency, and submission latency. Avoid secrets and high-cardinality metric labels.

**Completion gate:** a failed or uncertain execution can be investigated across request, persisted attempt, and external observation; metrics have documented meaning without fabricated benchmarks. Basic diagnostics should accompany earlier phases rather than wait for this gate.

## Phase 15 — AWS Deployment

**Status: foundation complete.** Docker packaging, ECS Fargate Terraform resources, CloudWatch logging, SSM secret references, health checks, and sanitized environment examples are present. A live AWS deployment, restore verification, and operational rollout are not claimed.

Package the Go service with Docker. Choose an appropriately sized AWS application service and PostgreSQL setup, with secrets management, migrations, health/readiness checks, backups, and observability. Document rollout, rollback, credential rotation, and unresolved-submission recovery. No Kubernetes is required.

**Completion gate:** controlled deployment and restart preserve run state; migration and restore procedures are verified; no secrets enter source control; external degradation does not erase durable work.

## Phase 16 — Basic Product UI

After backend correctness, add a minimal UI for projects, proposals, Run Diff, preflight results, reviewer actions, run state, and audit timeline. Show the exact reviewed revision, freshness, and submission uncertainty clearly. Keep authorization server-side.

**Completion gate:** a human can review and track the first vertical slice without hiding important state; stale UI actions cannot approve a different revision. The backend remains the primary engineering story.

## Phase 17 — Budget / Policy Controls

Expand deterministic CPU/memory bounds, approved revisions, allowed workflows, project budget thresholds, and reviewer requirements after execution is reliable. Define the source and uncertainty of any budget data before enforcement. Initial simple limits already belong in preflight/approval; this phase adds depth.

**Completion gate:** boundary cases and policy changes produce explainable, tested decisions. No advanced DSL, speculative cost forecasting, or ML-based estimation is required.

## Phase 18 — Agent-Native Interface

Only after human execution is reliable, consider scoped machine identities for proposing runs, retrieving diff/preflight, requesting approval, and querying execution status. Reuse domain services and audit attribution to the actual caller.

**Completion gate:** **software agents request; RunBridge authorizes.** Machines cannot bypass project permissions, required human decisions, or immutable intent. No LLM is required for the core product.

## Phase 19 — A2A Integration

Explore A2A-style agent/platform coordination only after the domain and authorization system are stable. A possible path is research agent → proposal → RunBridge preflight/policy → human approval when required → Seqera. This is an optional transport/interface investigation, outside the MVP.

**Completion gate:** any interface uses the same authenticated commands and deterministic authorization boundary; no alternate launch path is introduced. BioLab remains a separate project and is not an MVP dependency.

## Phase 20 — Additional Workflow Support

After nf-core/rnaseq is robust, assess one additional nf-core workflow. Derive shared concepts from real integration needs instead of creating a generic workflow engine first. Additional execution backends remain separate future scope decisions.

**Completion gate:** the original vertical slice retains its guarantees, workflow-specific validation remains explicit, and each abstraction has a demonstrated need.
