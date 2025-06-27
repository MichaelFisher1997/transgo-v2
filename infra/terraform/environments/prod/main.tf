data "aws_caller_identity" "current" {}

module "ecr" {
  source          = "../../modules/ecr"
  repository_name = "transgo-prod"
  tags = {
    Environment = "prod"
  }
}

module "s3" {
  source          = "../../modules/s3"
  bucket_name     = "transgo-media-prod"
  allowed_principals = [data.aws_caller_identity.current.arn]
  tags = {
    Environment = "prod"
  }
}

module "ecs" {
  source = "../../modules/ecs"

  cluster_name    = "transgo-cluster-prod"
  environment     = "prod"
  desired_count   = 2
  subnet_ids      = data.aws_subnets.default.ids
  target_group_arn = module.alb.target_group_arn
  security_group_id = module.alb.security_group_id
  ecr_repository_url = module.ecr.repository_url
  vpc_id         = data.aws_vpc.default.id
}

module "alb" {
  source = "../../modules/alb"

  environment = "prod"
  vpc_id      = data.aws_vpc.default.id
}
