# Deployment

RunBridge is packaged as a small, non-root distroless Go container. The AWS
Terraform under `terraform/` provisions the application on ECS Fargate with a
TLS load balancer, private encrypted RDS PostgreSQL, managed secrets,
least-privilege IAM, restricted security groups, CloudWatch logs, readiness
checks, backups, and rollback controls.

Nextflow execution remains in Seqera; RunBridge does not create a scheduler or
Kubernetes cluster. Its reconciliation and polling workers share the service
process while PostgreSQL retains durable work across restarts.

The checked-in infrastructure is locally validated but has not been applied to
an AWS account. Applying it creates billable resources. Review the cost warning
and operating runbook in [terraform/README.md](terraform/README.md) before any
real deployment.
