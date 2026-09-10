data "aws_partition" "current" {}

resource "aws_ecr_repository" "runbridge" {
  name                 = var.name
  image_tag_mutability = "IMMUTABLE"
  encryption_configuration { encryption_type = "AES256" }
  image_scanning_configuration { scan_on_push = true }
}

resource "aws_ecr_lifecycle_policy" "runbridge" {
  repository = aws_ecr_repository.runbridge.name
  policy     = jsonencode({ rules = [{ rulePriority = 1, description = "Retain recent images", selection = { tagStatus = "untagged", countType = "imageCountMoreThan", countNumber = 5 }, action = { type = "expire" } }] })
}

resource "aws_cloudwatch_log_group" "runbridge" {
  name              = "/ecs/${var.name}"
  retention_in_days = var.log_retention_days
}

resource "aws_ecs_cluster" "runbridge" {
  name = var.name
  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

resource "aws_iam_role" "execution" {
  name               = "${var.name}-ecs-execution"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { Service = "ecs-tasks.amazonaws.com" }, Action = "sts:AssumeRole" }] })
}

resource "aws_iam_role_policy_attachment" "execution" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "secrets" {
  name = "read-runbridge-secrets"
  role = aws_iam_role.execution.id
  policy = jsonencode({ Version = "2012-10-17", Statement = concat([
    { Effect = "Allow", Action = ["ssm:GetParameter", "ssm:GetParameters"], Resource = [var.seqera_token_parameter_arn, var.api_token_parameter_arn, var.webhook_secret_parameter_arn] },
    { Effect = "Allow", Action = ["secretsmanager:GetSecretValue"], Resource = [aws_db_instance.runbridge.master_user_secret[0].secret_arn] }
  ], length(var.secret_kms_key_arns) == 0 ? [] : [{ Effect = "Allow", Action = ["kms:Decrypt"], Resource = var.secret_kms_key_arns }]) })
}

resource "aws_iam_role" "task" {
  name               = "${var.name}-task"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { Service = "ecs-tasks.amazonaws.com" }, Action = "sts:AssumeRole" }] })
}

resource "aws_security_group" "load_balancer" {
  name_prefix = "${var.name}-alb-"
  description = "TLS ingress to RunBridge"
  vpc_id      = var.vpc_id
  lifecycle { create_before_destroy = true }
}

resource "aws_security_group" "service" {
  name_prefix = "${var.name}-service-"
  description = "RunBridge application tasks"
  vpc_id      = var.vpc_id
  lifecycle { create_before_destroy = true }
}

resource "aws_security_group" "database" {
  name_prefix = "${var.name}-database-"
  description = "PostgreSQL from RunBridge tasks"
  vpc_id      = var.vpc_id
  lifecycle { create_before_destroy = true }
}

resource "aws_security_group_rule" "public_to_load_balancer" {
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = var.ingress_cidr_blocks
  security_group_id = aws_security_group.load_balancer.id
}

resource "aws_security_group_rule" "load_balancer_to_service" {
  type                     = "egress"
  from_port                = 8080
  to_port                  = 8080
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.service.id
  security_group_id        = aws_security_group.load_balancer.id
}

resource "aws_security_group_rule" "service_from_load_balancer" {
  type                     = "ingress"
  from_port                = 8080
  to_port                  = 8080
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.load_balancer.id
  security_group_id        = aws_security_group.service.id
}

resource "aws_security_group_rule" "service_to_https" {
  type              = "egress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.service.id
}

resource "aws_security_group_rule" "service_to_database" {
  type                     = "egress"
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.database.id
  security_group_id        = aws_security_group.service.id
}

resource "aws_security_group_rule" "database_from_service" {
  type                     = "ingress"
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.service.id
  security_group_id        = aws_security_group.database.id
}

resource "aws_db_subnet_group" "runbridge" {
  name       = var.name
  subnet_ids = var.private_subnet_ids
}

resource "aws_db_instance" "runbridge" {
  identifier                   = var.name
  engine                       = "postgres"
  engine_version               = var.postgres_engine_version
  instance_class               = var.database_instance_class
  allocated_storage            = var.database_storage_gib
  max_allocated_storage        = var.database_max_storage_gib
  storage_encrypted            = true
  db_name                      = var.database_name
  username                     = var.database_username
  manage_master_user_password  = true
  db_subnet_group_name         = aws_db_subnet_group.runbridge.name
  vpc_security_group_ids       = [aws_security_group.database.id]
  publicly_accessible          = false
  backup_retention_period      = var.database_backup_retention_days
  deletion_protection          = var.database_deletion_protection
  skip_final_snapshot          = !var.database_deletion_protection
  final_snapshot_identifier    = var.database_deletion_protection ? "${var.name}-final" : null
  auto_minor_version_upgrade   = true
  performance_insights_enabled = true
  apply_immediately            = false
}

resource "aws_lb" "runbridge" {
  name               = var.name
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.load_balancer.id]
  subnets            = var.public_subnet_ids
}

resource "aws_lb_target_group" "runbridge" {
  name        = var.name
  port        = 8080
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = var.vpc_id
  health_check {
    path                = "/readyz"
    matcher             = "200"
    interval            = 30
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.runbridge.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.certificate_arn
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.runbridge.arn
  }
}

resource "aws_ecs_task_definition" "runbridge" {
  family                   = var.name
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  container_definitions = jsonencode([{
    name         = var.name, image = var.image, essential = true, readonlyRootFilesystem = true,
    portMappings = [{ containerPort = 8080, hostPort = 8080, protocol = "tcp" }],
    secrets = [
      { name = "DB_PASSWORD", valueFrom = "${aws_db_instance.runbridge.master_user_secret[0].secret_arn}:password::" },
      { name = "SEQERA_TOKEN", valueFrom = var.seqera_token_parameter_arn },
      { name = "RUNBRIDGE_API_TOKEN", valueFrom = var.api_token_parameter_arn },
      { name = "RUNBRIDGE_WEBHOOK_SECRET", valueFrom = var.webhook_secret_parameter_arn }
    ],
    environment = [
      { name = "DB_HOST", value = aws_db_instance.runbridge.address },
      { name = "DB_PORT", value = tostring(aws_db_instance.runbridge.port) },
      { name = "DB_NAME", value = var.database_name },
      { name = "DB_USER", value = var.database_username },
      { name = "DB_SSLMODE", value = "require" },
      { name = "RUNBRIDGE_ACTOR_ID", value = var.actor_id },
      { name = "RUNBRIDGE_ACTOR_NAME", value = var.actor_name },
      { name = "SEQERA_BASE_URL", value = var.seqera_base_url }
    ],
    logConfiguration = { logDriver = "awslogs", options = { "awslogs-group" = aws_cloudwatch_log_group.runbridge.name, "awslogs-region" = var.aws_region, "awslogs-stream-prefix" = "runbridge" } }
  }])
}

resource "aws_ecs_service" "runbridge" {
  name                              = var.name
  cluster                           = aws_ecs_cluster.runbridge.arn
  task_definition                   = aws_ecs_task_definition.runbridge.arn
  desired_count                     = var.desired_count
  launch_type                       = "FARGATE"
  health_check_grace_period_seconds = 60
  enable_execute_command            = false
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.service.id]
    assign_public_ip = false
  }
  load_balancer {
    target_group_arn = aws_lb_target_group.runbridge.arn
    container_name   = var.name
    container_port   = 8080
  }
  depends_on = [aws_lb_listener.https, aws_iam_role_policy.secrets]
}
