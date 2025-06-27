data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

variable "db_password" {
  description = "Database administrator password"
  type        = string
  sensitive   = true
  default     = "postgres123"
}
