# Audit model

PostgreSQL stores append-oriented audit evidence alongside lifecycle records. Aggregate writes append proposal/specification, review, approval, execution, attempt, reconciliation, external observation, transition, artifact, and receipt events in the same transaction where correctness requires it. The authorized timeline API uses bounded cursor pagination and project isolation.

## Event envelope

| Field | Purpose |
| --- | --- |
| Event identity and schema version | Stable reference and interpretable evolution |
| Actor identity and actor kind | Human, internal coordinator, or authenticated external source |
| Recorded timestamp | When RunBridge durably accepted the event |
| Source occurrence timestamp, when available | When an external action reportedly occurred |
| Project and run references | Isolation and timeline ownership |
| Event type | Meaningful domain action or observation |
| Relevant object / revision references | Specification, baseline, diff, preflight, policy, approval, attempt, artifact |
| Correlation and source identifiers | Request, submission attempt, external execution, and delivery deduplication |
| Metadata | Structured reasons, prior/new state, and minimal relevant evidence |

PostgreSQL provides the current storage schema and stable local ordering. Timestamps alone do not establish total ordering across systems, so arrival order remains distinct from source-reported time.

## Timeline coverage

Meaningful events include proposal creation/revision, diff calculation, preflight completion, policy evaluation, approval request, approval grant/rejection/invalidation, launch intent, submission uncertainty, external ID assignment, execution observation, state transition, cancellation request/result, execution completion, and artifact recording.

Approval evidence identifies the exact normalized specification and review context. Submission evidence identifies the approved revision, attempt, translated request reference, and external Seqera execution ID when known. Preserve uncertainty rather than manufacturing a successful submission event before acceptance is established.

## Persistence and immutability

Commit local state changes and their audit events in one transaction. Separate external delivery records from meaningful domain events so redelivery does not fabricate repeated transitions. This is an audit timeline alongside state tables, not a requirement to build a full event-sourced application.

Application paths should append events, with database privileges and schema constraints later limiting accidental update/deletion. Corrections reference earlier events through new records. Administrative database access can still alter data; append-oriented storage alone is not cryptographic tamper proofing. Define retention, backups, privileged access, and recovery procedures before deployment.

## Artifacts and integrity

Artifact records identify location, provenance, external execution, and available version/size/content evidence without copying large scientific datasets into the control-plane database. Canonical manifests and SHA-256 hashes bind recorded outputs to execution receipts; chained audit hashes and KMS signatures are optional hardening. Missing or unverified artifacts must remain explicitly marked, and artifact collection failure must not rewrite a confirmed execution outcome.

## Access and data minimization

Audit queries and artifact access must enforce project permissions. Do not persist credentials, bearer tokens, signed access URLs, or unbounded external payloads in audit metadata. Scientific input names and locations may be sensitive; retain sufficient references to trace execution with access controls and deliberate retention. Operational logs supplement this evidence but are not the audit record.
