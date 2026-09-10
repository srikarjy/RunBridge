# RunBridge deployment operations

This runbook describes the controls around the ECS and PostgreSQL deployment
slice. It assumes the environment supplies a VPC, public/private subnets, an
ACM certificate, private-subnet egress, and three SSM SecureString parameters.
Terraform creates the ECS roles, encrypted PostgreSQL service, and load
balancer. Applying it creates billable resources.

## Release and migration order

1. Build the image with the immutable commit or digest recorded in the release
   change.
2. Push the image to the immutable ECR repository and update `image` with its
   digest in the environment variable file held outside Git.
3. Confirm the generated execution role can read only the named SSM parameters
   and RDS-managed password, and review all security-group paths.
4. Apply Terraform. The first task runs embedded migrations before it serves
   readiness, so a failed migration keeps the service out of rotation.
5. Verify `/healthz`, `/readyz`, `/metrics`, and an authenticated project-scoped
   read before considering the rollout complete.

Migrations are ordered and recorded in `schema_migrations`. They must remain
backward compatible with the previous application during a rolling deployment.
Never edit an applied migration; add a new numbered file.

## Rollback

ECS deployment circuit-breaker rollback is enabled. If the new task fails its
health check or cannot connect to PostgreSQL, ECS returns to the last healthy
task definition. Roll back the image and application release together; do not
reverse an applied database migration unless a reviewed down migration exists.

## Secrets and identity

`SEQERA_TOKEN`, `RUNBRIDGE_API_TOKEN`, and `RUNBRIDGE_WEBHOOK_SECRET` are read
from SSM SecureString parameters. RDS manages the database master password in
Secrets Manager. Rotate an application secret by writing a new parameter
value, forcing a new ECS deployment, checking
the authenticated health path, and then revoking the old value. Values never
belong in Terraform state files, logs, audit metadata, or environment files
committed to the repository.

## PostgreSQL backup and restore

Use the managed PostgreSQL service's encrypted automated backups and periodic
restore drills. A restore is accepted only after migrations run successfully,
project memberships and execution rows are present, and an audit timeline can
be read. Restore verification should include an execution in
`SUBMISSION_UNKNOWN`; its attempt and correlation records must survive so an
operator can reconcile it before retrying.

## Uncertain submissions

After a restart, inspect recoverable `SUBMITTING`, `SUBMISSION_UNKNOWN`, and
`RUNNING` executions. Query Seqera using the persisted workspace and attempt
correlation before retrying. A single correlated match can be adopted; no match
without definitive rejection remains unknown; multiple matches require manual
review. Do not use a network timeout as proof that no workflow was accepted.

## Observability

Review structured logs by request and run correlation IDs and inspect the
Prometheus-compatible counters for HTTP errors, submission failures,
reconciliation attempts, duplicate webhooks, and transition conflicts. Alerts
and dashboards are environment-specific and should be added without placing
secrets or unbounded identifiers in metric labels.
