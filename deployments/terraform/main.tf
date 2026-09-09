resource "aws_cloudwatch_log_group" "runbridge" {
  name              = "/ecs/${var.name}"
  retention_in_days = 30
}

resource "aws_ecs_task_definition" "runbridge" {
  family                   = var.name
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = var.execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([{
    name         = var.name
    image        = var.image
    essential    = true
    portMappings = [{ containerPort = 8080, hostPort = 8080, protocol = "tcp" }]
    secrets = [
      { name = "DATABASE_URL", valueFrom = var.database_url_parameter_arn },
      { name = "SEQERA_TOKEN", valueFrom = var.seqera_token_parameter_arn }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.runbridge.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "runbridge"
      }
    }
    healthCheck = {
      command     = ["CMD-SHELL", "wget -qO- http://localhost:8080/healthz || exit 1"]
      interval    = 30
      timeout     = 5
      retries     = 3
      startPeriod = 10
    }
  }])
}

resource "aws_ecs_service" "runbridge" {
  name            = var.name
  cluster         = var.cluster_arn
  task_definition = aws_ecs_task_definition.runbridge.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = var.security_group_ids
    assign_public_ip = false
  }
}
