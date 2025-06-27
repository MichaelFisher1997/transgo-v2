variable "bucket_name" {
  description = "Name of the S3 bucket"
  type        = string
}

variable "retention_days" {
  description = "Number of days to retain media files"
  type        = number
  default     = 90
}

variable "allowed_principals" {
  description = "List of IAM principals allowed to access the bucket"
  type        = list(string)
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}
