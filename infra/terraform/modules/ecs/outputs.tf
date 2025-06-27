output "cluster_name" {
  description = "Name of the ECS cluster"
  value       = aws_ecs_cluster.transgo.name
}

output "service_name" {
  description = "Name of the ECS service"
  value       = aws_ecs_service.transgo.name
}

output "task_definition_arn" {
  description = "ARN of the ECS task definition"
  value       = aws_ecs_task_definition.transgo.arn
}

output "security_group_id" {
  description = "Security group ID for ECS tasks"
  value       = aws_security_group.ecs.id
}
