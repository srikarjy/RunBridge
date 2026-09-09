# ADR 0014: Start with Vendor-Neutral Reliability Counters

## Status

Accepted for Phase 14 foundation.

## Decision

RunBridge records a small set of atomic process counters for submission failures, reconciliation attempts, duplicate webhooks, and transition conflicts. A snapshot is cheap to export later to Prometheus, CloudWatch, or another metrics system.

## Consequences

Instrumentation can be added before selecting a telemetry vendor. Labels, histograms, tracing, dashboards, and restart-persistent aggregation remain deployment concerns.

