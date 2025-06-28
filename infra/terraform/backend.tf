terraform {
  backend "s3" {
    bucket         = "transgo-media-backend-064bcea"
    key            = "${var.branch_name}/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "terraform-locks"  # If using state locking
    encrypt        = true
  }
}

variable "branch_name" {
  description = "Name of the branch for Terraform state key"
  type        = string
}
