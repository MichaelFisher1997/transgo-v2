data "aws_caller_identity" "current" {}

module "ecr" {
  source          = "../../modules/ecr"
  repository_name = "transgo-dev"
  tags = {
    Environment = "dev"
  }
}

module "s3" {
  source          = "../../modules/s3"
  bucket_name     = "transgo-media-dev"
  allowed_principals = [data.aws_caller_identity.current.arn]
  tags = {
    Environment = "dev"
  }
}

module "ecs" {
  source = "../../modules/ecs"

  cluster_name    = "transgo-cluster-dev"
  environment     = "dev"
  desired_count   = 1
  subnet_ids      = data.aws_subnets.default.ids
  target_group_arn = module.alb.target_group_arn
  security_group_id = module.alb.security_group_id
  ecr_repository_url = module.ecr.repository_url
  vpc_id         = data.aws_vpc.default.id
}

module "alb" {
  source = "../../modules/alb"

  environment = "dev"
  vpc_id      = data.aws_vpc.default.id
}

module "rds" {
  source = "../../modules/rds"

  db_password       = var.db_password
  security_group_id = module.alb.security_group_id
}
