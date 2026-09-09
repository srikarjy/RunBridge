# ADR 0012: Hash Exact Normalized Specification Bytes

## Status

Accepted for Phase 13 foundation.

## Decision

RunBridge derives a SHA-256 digest from length-prefixed workflow name, workflow revision, normalization version, and the exact normalized document bytes persisted in PostgreSQL. The digest is a content identity for approval/execution correspondence; it is not a signature.

## Consequences

Equivalent canonical specifications produce the same identity, while any byte change produces a different identity. AWS KMS signatures, artifact manifests, and receipt storage can be layered on later without changing the canonicalization contract.

