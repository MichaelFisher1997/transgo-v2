variable "environment" {
  description = "Environment name (dev/stage/prod)"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID where ALB should be created"
  type        = string
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }
}
