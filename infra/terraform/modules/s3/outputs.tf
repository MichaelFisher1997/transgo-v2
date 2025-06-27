output "bucket_arn" {
  description = "The ARN of the bucket"
  value       = data.aws_s3_bucket.media.arn
}

output "bucket_name" {
  description = "The name of the bucket"
  value       = data.aws_s3_bucket.media.id
}

output "bucket_domain_name" {
  description = "The domain name of the bucket"
  value       = data.aws_s3_bucket.media.bucket_domain_name
}

output "bucket_regional_domain_name" {
  description = "The regional domain name of the bucket"
  value       = data.aws_s3_bucket.media.bucket_regional_domain_name
}
