provider "aws" {
  region = var.aws_region

  # Use environment variables for credentials in CI/CD
  # AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY should be set
  # in GitHub Secrets for the workflow
}

variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-east-1"
}
