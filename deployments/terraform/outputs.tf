output "service_url" {
  value = "https://${aws_lb.runbridge.dns_name}"
}

output "task_definition_arn" {
  value = aws_ecs_task_definition.runbridge.arn
}

output "service_name" {
  value = aws_ecs_service.runbridge.name
}

output "cluster_arn" {
  value = aws_ecs_cluster.runbridge.arn
}

output "database_address" {
  value = aws_db_instance.runbridge.address
}

output "database_master_secret_arn" {
  value     = aws_db_instance.runbridge.master_user_secret[0].secret_arn
  sensitive = true
}

output "log_group_name" {
  value = aws_cloudwatch_log_group.runbridge.name
}

output "ecr_repository_url" {
  value = aws_ecr_repository.runbridge.repository_url
}
