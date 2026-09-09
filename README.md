# RunBridge

**Preflight, approval, and audit infrastructure for expensive computational workflows.**

RunBridge is a planned Go control plane between a person requesting a scientific workflow and the platform that executes it. Its first vertical slice will support **nf-core/rnaseq through Seqera / Nextflow**, with PostgreSQL as the system of record.

**Current status: Phase 4 — nf-core/rnaseq specification.** The repository contains domain types, constrained PostgreSQL persistence, project authorization, and deterministic normalization for the first supported workflow. No HTTP authentication adapter, API, Seqera integration, Run Diff, policy engine, lifecycle transition engine, or runnable application exists yet. All other capabilities below describe intended behavior.

## The Problem

Computational workflows combine large datasets, expensive cloud compute, complex parameters, workflow and container revisions, scientific assumptions, and human approvals. A small configuration mistake can waste compute, cause unexpected cost, invalidate an analysis, or make results difficult to reproduce.

Workflow engines answer: **How do I execute this workflow?**

RunBridge asks: **Should this exact workflow configuration be executed, who approved it, and can we prove what actually ran?**

The initial users are bioinformatics engineers submitting runs, scientists reviewing configuration changes, team leads approving execution, and platform engineers responsible for reliable operations. RNA sequencing is the initial application domain; the engineering focus is APIs, authorization, persistence, state, and external-system reliability.

## What RunBridge Does

```text
Propose → Diff → Preflight → Approve → Execute → Reconcile → Audit
```

1. **Propose:** capture a user's intended workflow, inputs, configuration, and project.
2. **Diff:** normalize the specification and compare it with a selected baseline or prior approved run.
3. **Preflight:** return structured results for deterministic scientific and configuration checks.
4. **Approve:** evaluate policy and obtain a reviewer decision when required, bound to the exact specification.
5. **Execute:** submit the frozen, authorized specification to Seqera.
6. **Reconcile:** resolve external state through polling and execution events, including uncertain submissions.
7. **Audit:** preserve the evidence connecting intent, approval, submission, and outcome.

The flow describes the review experience, not independent unguarded API calls. Both diff and preflight must reference the same normalized specification before approval; audit events will be recorded throughout.

## Run Diff

**Run Diff is a first-class planned feature.** It will compare a proposal with a previous run, project baseline, approved configuration, or another accessible specification.

Meaningful differences include workflow revision, parameters, samples and input references, reference genome, requested CPUs and memory, containers, and execution configuration. A reference genome is the reference sequence used to interpret the input data; changing it can change the interpretation of an analysis.

Comparing JSON strings cannot reliably distinguish formatting changes from scientific or resource changes. RunBridge will normalize supported fields, preserve meaningful distinctions, and produce structured differences that can drive both human review and deterministic policy. Missing values, explicit defaults, units, and order-sensitive fields need defined comparison semantics. Unknown fields must not silently disappear.

Run Diff is not implemented. See [architecture](docs/architecture.md) and [Phase 6](ROADMAP.md#phase-6--run-diff).

## Preflight

Planned checks cover required fields, supported workflow and revision, sample input structure, project access, accessible datasets where verifiable, reference configuration conflicts, and resource boundaries. Results will identify the check, affected field, severity, and reason. Preflight cannot prove scientific correctness or guarantee that remotely referenced inputs remain unchanged.

## Approval

**Approval applies to a specific normalized run specification.** Changes to execution-relevant intent will require renewed validation, diff, policy evaluation, and approval when required. A mutable proposal and an immutable approved revision are separate concepts.

The invariant is: **the run that executes must correspond to the run specification that was approved.** Simple code/config-driven rules will determine approval requirements. Policy-permitted runs that do not need human review will still have an explicit authorization decision. See the [approval model](docs/approval-model.md).

## Execution

The initial execution backend is Seqera, running Nextflow and nf-core/rnaseq. RunBridge will sit above that platform; Nextflow will remain responsible for workflow execution. A narrow integration boundary will translate the frozen specification into the external launch request and retain execution identifiers and submission evidence.

## Durable State

Run state must survive server restarts, repeated client requests, duplicate webhooks, network failures, and retries. PostgreSQL will persist transitions and the evidence needed to resume work.

An especially important case is **SUBMISSION_UNKNOWN**: Seqera may accept a launch while RunBridge loses the response. Retrying immediately could launch another expensive run. RunBridge must retain the uncertain attempt and reconcile against external evidence before deciding whether another submission is safe. The [lifecycle](docs/run-lifecycle.md) and [reliability design](docs/reliability.md) describe this boundary.

## Auditability

The planned chronological event history will record the proposal, diff, preflight result, policy decision, approval, approved specification, submission attempt, external execution ID, state transitions, relevant artifacts, and final status.

It should answer: **Who requested what, what changed, who approved it, what executed, and what happened?** Audit evidence will reference immutable revisions and distinguish user actions, internal coordination, and external observations. See the [audit model](docs/audit-model.md).

## System Architecture

This is a target design, not a deployed system.

```mermaid
flowchart TD
    U[User / Reviewer] --> API[RunBridge REST API]
    API --> PA[Projects + Authorization]
    PA --> RP[Run Proposal]
    RP --> N[Normalization]
    N --> DP[Run Diff + Preflight]
    DP --> AP[Policy + Approval]
    AP --> F[Frozen Approved Specification]
    F --> E[Execution Service]
    E --> S[Seqera / Nextflow]
    S --> RNA[nf-core/rnaseq]
    S --> EV[Webhook / Polling Observations]
    EV --> R[Reconciliation]
    R --> DA[Durable State + Audit Timeline]
    RP <--> DB[(PostgreSQL: System of Record)]
    AP <--> DB
    E <--> DB
    DA <--> DB
```

The intended starting architecture is one Go service with structured domain boundaries and PostgreSQL. Background coordination can run within the same deployment, using durable database records rather than a separate message queue. See [architecture details](docs/architecture.md).

## Example Scenario

**Illustrative only: these are hypothetical values, not actual project runs or execution results.**

| Field | Previous specification | Proposed specification |
| --- | --- | --- |
| Samples | 24 | 72 |
| Requested memory | 16 GB | 64 GB |
| Requested CPUs | 8 | 32 |

RunBridge would surface these changes before execution and evaluate the configured review requirements. Resource units and whether a limit applies to an individual task or the run context must be explicit; these example values do not imply a cost estimate or a universal Nextflow resource field.

## Reliability Challenges

The execution boundary requires more than CRUD: duplicate submissions, crashes between database updates and external calls, webhook redelivery, eventual consistency, API outages, and cancellation racing with completion all affect correctness. Planned controls include scoped idempotency keys, transactional state changes, bounded retries, persistent submission attempts, and reconciliation. An HTTP timeout is not proof that a launch failed, and a cancellation request is not proof that execution stopped.

## Security / Authorization

Planned server-side authorization will enforce project memberships and roles for every command and query, including baseline selection, approvals, artifacts, and audit access. Viewer, runner, reviewer, and admin are conceptual roles pending implementation. Approval records will preserve reviewer identity and immutable targets; membership changes and submission eligibility will be rechecked at the execution boundary. Machine identities may be added later. Credentials belong in secret storage, not source control or audit payloads.

## Reproducibility / Integrity

The design calls for a canonical normalized specification and immutable approval target. Later integrity work will add SHA-256 specification hashes, artifact manifests, approval receipts, and final execution receipts, with potential AWS KMS-backed signatures.

Hashes attest to recorded bytes, not scientific validity or the contents behind a mutable URL. Pinned workflow/container references and versioned input evidence will be necessary to strengthen reproducibility. Stage 1 implements no cryptography.

## Initial Scope

### MVP

- nf-core/rnaseq only, through Seqera / Nextflow.
- Human-driven proposals and project-level authorization.
- Run Diff, deterministic preflight, simple approval rules, and reviewer workflow.
- Durable execution lifecycle, submission reconciliation, and audit history.
- Observability and basic deployment.

### Later

Additional nf-core workflows, machine identities, agent-submitted proposals, an A2A interface, expanded cost/budget controls, and potentially other execution backends may follow a reliable first vertical slice. None exists today.

## AI / Agent Philosophy

RunBridge is designed so that AI systems may eventually propose or explain actions, while deterministic software remains responsible for authorization and execution. Possible uses include explaining Run Diff, summarizing preflight failures, and helping construct proposals. **AI does not approve or directly bypass policy.** The MVP must work fully without an LLM; A2A is deferred. BioLab is a separate project, with no code sharing or integration in this scope.

## Technology

**Present:** Go domain packages for human actors, projects, memberships, proposals, immutable specification revisions, workflow identity, normalized configuration values, and run status vocabulary. The authorization package resolves project membership and evaluates explicit permissions. `internal/runs/rnaseq` defines and canonically normalizes the first workflow-specific request. PostgreSQL migrations define durable project, proposal, approval, execution, attempt, and audit structures. A small `pgx`-backed store persists current actors, projects, memberships, proposals, and revisions. There is no runtime command.

**Planned core:** Go, REST, PostgreSQL, Seqera API, Nextflow, and nf-core/rnaseq.

**Planned engineering:** context-aware request handling, idempotent operations, durable state transitions, RBAC, external reconciliation, structured logging, metrics, useful tracing, Docker, and an appropriately sized AWS deployment.

**Later integrity hardening:** SHA-256 specifications and manifests, with AWS KMS signing considered after the execution path is reliable.

## Repository Structure

| Location | Intended responsibility |
| --- | --- |
| `cmd/runbridge/` | Future service entry point and dependency wiring |
| `internal/auth/`, `internal/projects/` | Identity, permissions, memberships, and project isolation |
| `internal/authorization/` | Project-scoped role and permission evaluation |
| `internal/runs/` | Proposals, specification revisions, normalization, and domain invariants |
| `internal/runs/rnaseq/` | nf-core/rnaseq request validation and deterministic normalization |
| `internal/preflight/`, `internal/rundiff/` | Deterministic validation and semantic comparisons |
| `internal/policy/`, `internal/approvals/` | Rules, reviewer decisions, and immutable approval targets |
| `internal/execution/` | Durable coordination, retries, cancellation, and reconciliation |
| `internal/postgres/` | Embedded PostgreSQL migrations and persistence adapters |
| `internal/audit/`, `internal/observability/` | Event history and operational signals |
| `integrations/seqera/` | External API translation and behavior |
| `configs/`, `deployments/` | Configuration guidance and future deployment direction |
| `docs/` | Architecture, lifecycle, approval, audit, and reliability designs |
| `tests/integration/`, `tests/fixtures/` | Future integration verification and sanitized fixture data |

Empty `.gitkeep` files retain the remaining planned directories in Git; they are not implemented packages. This layout is provisional and may be simplified as the first vertical slice reveals real boundaries.

## Current Status

**Phase 4 — nf-core/rnaseq specification.** The first workflow-specific request model now produces deterministic versioned normalized bytes for samples, references, parameters, profile, and resources. Validation is structural; preflight, Run Diff, policy, API authentication, and workflow execution remain future work.

Run unit checks with `go test ./...`. PostgreSQL integration tests run when `RUNBRIDGE_TEST_DATABASE_URL` points to a dedicated test database; each test creates and removes its own schema.

## Roadmap

The [full roadmap](ROADMAP.md) progresses from domain modeling, PostgreSQL, and authorization to the rnaseq specification, preflight, Run Diff, and approval. It then covers Seqera integration, durable execution, reconciliation, webhooks, audit access, integrity, observability, and deployment. UI, expanded budget controls, agents, A2A, and additional workflows follow later. Phases are gates, not delivery dates.

## Non-Goals

RunBridge is not a replacement for Nextflow or Seqera, a new workflow scheduler, a generic orchestration engine, a computational biology algorithm, a new LLM framework, an autonomous scientific agent, a RAG system, an MCP server, or a Kubernetes platform.

The MVP excludes custom compute infrastructure, advanced policy DSLs, ML cost forecasting, recommendation engines, vector databases, agent orchestration, and chaos engineering. It does not need Kafka, Redis, microservices, a service mesh, custom distributed queues, or LLM orchestration. Add infrastructure only when demonstrated requirements justify it.

Its contribution is **governance, approval, reliable execution control, and auditability around computational workflows.**

## License

[MIT License](LICENSE).
