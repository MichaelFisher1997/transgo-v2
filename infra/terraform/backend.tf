terraform {
  backend "s3" {
    bucket         = "transgo-media-backend-064bcea"
    key            = "terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "terraform-locks"  # If using state locking
    encrypt        = true
  }
}
