variable "cluster_name" {
  description = "Name of the ECS cluster"
  type        = string
}

variable "environment" {
  description = "Environment name (dev/stage/prod)"
  type        = string
}

variable "desired_count" {
  description = "Number of ECS tasks to run"
  type        = number
  default     = 1
}

variable "subnet_ids" {
  description = "List of subnet IDs for ECS service"
  type        = list(string)
}

variable "target_group_arn" {
  description = "ARN of the load balancer target group"
  type        = string
}

variable "security_group_id" {
  description = "Security group ID for ECS tasks"
  type        = string
}

variable "ecr_repository_url" {
  description = "ECR repository URL for container images"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID for ECS resources"
  type        = string
}
