# ADR 0011: Verify Webhook Signatures Before Parsing

## Status

Accepted as a security foundation.

## Decision

Integration adapters may use the vendor-neutral verifier in `internal/security` to authenticate raw webhook bytes. The signature covers `timestamp + "." + body`, uses HMAC-SHA256 and constant-time comparison, and rejects timestamps outside a configured replay window. Vendor-specific headers and secret loading remain at the HTTP adapter boundary.

## Consequences

Tampered or replayed requests can be rejected before event decoding. This does not replace TLS, secret rotation, authorization, delivery persistence, or Seqera-specific verification requirements.

