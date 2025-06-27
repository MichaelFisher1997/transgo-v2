variable "db_password" {
  description = "Database administrator password"
  type        = string
  sensitive   = true
}

variable "security_group_id" {
  description = "Security group ID for RDS access"
  type        = string
}
