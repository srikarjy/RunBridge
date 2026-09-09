# Deployment direction

The repository now includes a reviewable Terraform foundation under
`deployments/terraform/` for a Go service packaged with Docker and deployed to
AWS ECS Fargate. It expects existing networking, IAM roles, an ECS cluster, and
SSM Parameter Store references for PostgreSQL and Seqera credentials. Nextflow
execution remains the responsibility of the configured Seqera environment.

Choose an appropriately sized application service and PostgreSQL deployment once runtime needs are known. Do not introduce Kubernetes, a custom scheduler, or speculative infrastructure. Background coordination may initially share the application deployment while PostgreSQL retains durable work state.

Before deployment, define secrets management, least-privilege access, TLS/network boundaries, database migrations, backups and restore verification, health/readiness checks, structured logs, metrics, and operational ownership. Avoid requiring Seqera availability for process liveness; report integration degradation separately. Graceful shutdown and rollout must preserve incomplete submission attempts for recovery.

Migration and rollback procedures must respect immutable approval/audit evidence and compatible state readers. Credential rotation, retained audit data, and unresolved submission recovery need documented operating procedures. No Dockerfiles, cloud resources, credentials, or infrastructure code are created in this stage.
