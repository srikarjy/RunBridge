# RunBridge AWS deployment foundation

This Terraform configuration describes a small ECS Fargate service for a
containerized RunBridge application. It expects an existing ECS cluster,
private subnets, security groups, IAM roles, and SSM Parameter Store entries.
Those shared resources are inputs so this repository does not create a public
network or invent production credentials.

The task injects `DATABASE_URL` and `SEQERA_TOKEN` from SSM Parameter Store,
writes structured container logs to CloudWatch, and uses a `/healthz` check.
Use an immutable image digest in production. Database migrations, TLS ingress,
backups, alarms, and IAM policy documents require environment-specific review.

Example:

```sh
terraform init
terraform plan -var-file=environment.tfvars
terraform apply -var-file=environment.tfvars
```

Never commit `*.tfvars`, tokens, or generated Terraform state.
