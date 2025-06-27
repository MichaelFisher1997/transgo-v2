data "aws_s3_bucket" "media" {
  bucket = "transgo-media-backend-064bcea"
}

resource "aws_s3_bucket_lifecycle_configuration" "media" {
  bucket = data.aws_s3_bucket.media.id

  rule {
    id     = "media-retention"
    status = "Enabled"

    filter {
      prefix = ""
    }

    expiration {
      days = var.retention_days
    }
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "media" {
  bucket = data.aws_s3_bucket.media.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_versioning" "media" {
  bucket = data.aws_s3_bucket.media.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_policy" "media" {
  bucket = data.aws_s3_bucket.media.id
  policy = data.aws_iam_policy_document.bucket_policy.json
}

data "aws_iam_policy_document" "bucket_policy" {
  statement {
    actions   = ["s3:*"]
    resources = [
      data.aws_s3_bucket.media.arn,
      "${data.aws_s3_bucket.media.arn}/*"
    ]
    principals {
      type        = "AWS"
      identifiers = var.allowed_principals
    }
  }
}
