# Approval model

Stage 1 design only. **Approve immutable intent, not mutable UI state.**

## Approval target

The target is a specific normalized rnaseq specification revision: workflow and revision, input identities, parameters, references, resource intent, and execution-relevant environment. Normalization must define units, defaults, ordering, omitted values, and serialization semantics. Record the normalization version. Unknown execution-relevant values must be surfaced, rejected, or explicitly represented rather than silently discarded.

An editable proposal points to revisions; an approval points directly to one immutable revision. The execution coordinator must read that approved revision, never whatever happens to be the latest proposal at launch time. Mutable external references need pinning or evidence of resolved identity where supported.

## Decision evidence

A future approval record will identify the approval, project, run, specification revision, actor/reviewer, decision, timestamp, reason, policy version, diff and baseline identities, and preflight result. The decision can approve, reject, or record a deterministic no-review authorization. A human approval must never be fabricated for the no-review path.

Policy evaluates supported workflows/revisions, project permissions, resource boundaries, accessible inputs, significant changes, and reviewer requirements. Start with simple deterministic code/config rules. Failed mandatory validation or policy denial cannot be bypassed by approving a screen.

## Changes and concurrency

Any execution-relevant change creates a new revision and invalidates the old revision's authorization for the new intent. Old approval records remain historical facts. A review command must name its expected revision and review context; reject stale decisions rather than attaching them to a newer revision.

Persist the approval decision and corresponding audit event together. The transition from APPROVED to SUBMITTING must ensure the same revision remains eligible, the approval has not been revoked or superseded, and submission has not already been claimed. Authorization, membership changes, policy changes, and validation freshness need explicit re-evaluation rules at this boundary. More restrictive current policy must not silently inherit a prior permissive decision.

Once a launch is in progress, withdrawing approval does not undo the external side effect. Record the withdrawal and coordinate cancellation if authorized; retain the original approval evidence.

## Human authorization

Server-side project permissions decide who may propose, review, launch, and cancel. Whether proposers may review their own runs is an explicit policy decision to finalize during implementation. Administrative privileges must not imply silent modification of historical approvals. Cross-project baselines and object references require authorization as well.

## Integrity evolution

Initial correctness relies on immutable revision identity, transactional guards, and submission derived from the approved data. Later SHA-256 hashes will bind canonical specification bytes to approval receipts and execution evidence. Artifact manifests and potentially AWS KMS signatures can strengthen verification. None is implemented in Stage 1, and hashing mutable URLs alone would not establish input reproducibility.

## Future machine proposals

Machine-generated proposals may eventually enter the same flow under scoped machine identities. Agents may propose actions or explain review evidence; deterministic RunBridge software owns permissions, policy, transitions, and execution. Human review requirements remain enforceable. A2A is a later interface, not an alternate authorization path.
