# RunBridge AWS deployment

This Terraform configuration is a deployable single-service AWS slice. Given
an existing VPC, two public subnets, two private subnets, and an ACM
certificate, it creates:

- an immutable, scan-on-push ECR repository;
- an ECS Fargate cluster, task definition, and service in private subnets;
- a public TLS application load balancer with `/readyz` target checks;
- an encrypted private RDS PostgreSQL instance with backups, deletion
  protection, storage autoscaling, and an AWS-managed master password;
- narrowly scoped security groups and separate ECS execution/task roles; and
- CloudWatch logs with Container Insights.

The ECS execution role can read only the supplied SSM parameters and the
RDS-managed database secret. The application container uses a read-only root
filesystem. Database credentials are injected as separate environment values,
and the service constructs a TLS-required connection string without storing a
password in Terraform source.

## Cost warning

**Do not run `terraform apply` just to validate this portfolio project.** RDS,
the load balancer, NAT gateway traffic required by private tasks, and running
Fargate tasks can all generate charges. `terraform fmt` and `terraform
validate` are local checks and create no AWS resources. No live deployment is
required to review the implementation.

For a real environment, start from `environment.tfvars.example`, restrict
`ingress_cidr_blocks`, use an immutable image digest, and keep the real tfvars
file outside source control. The VPC must provide private-subnet egress to the
Seqera API and the AWS services used during task startup.

```sh
terraform init -backend=false
terraform fmt -check
terraform validate
terraform plan -var-file=environment.tfvars
```

Only an operator who understands the resulting plan and cost should run
`terraform apply`. See [OPERATIONS.md](OPERATIONS.md) for rollout, rollback,
secret rotation, restore, and uncertain-submission recovery procedures.
