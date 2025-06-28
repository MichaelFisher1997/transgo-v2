# This is a template for environment-specific Terraform configurations.
# It will be copied and modified for each branch.

# Example: Define resources that are common across environments but might need
# slight variations based on the environment name (e.g., resource tagging).

# Example: VPC, ALB, ECR, ECS, RDS, S3 configurations would go here,
# parameterized by the 'environment' variable.

# Placeholder for common resources
# resource "aws_instance" "example" {
#   ami           = "ami-0c55b159cbfafe1f0" # Example AMI
#   instance_type = "t2.micro"
#   tags = {
#     Environment = var.environment
#   }
# }

# Common variables that will be set dynamically
variable "environment" {
  description = "Environment name (e.g., branch name)"
  type        = string
}

variable "aws_region" {
  description = "AWS region"
  type        = string
}

# Include common modules here, parameterized by environment
# module "ecs" {
#   source = "../../modules/ecs"
#   # ... other parameters ...
#   environment = var.environment
# }
