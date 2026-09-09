output "task_definition_arn" { value = aws_ecs_task_definition.runbridge.arn }
output "service_name" { value = aws_ecs_service.runbridge.name }
output "log_group_name" { value = aws_cloudwatch_log_group.runbridge.name }
