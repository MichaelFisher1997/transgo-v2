# CI/CD Workflows Documentation

This directory contains GitHub Actions workflows that automate our build, test, and deployment processes.

## Workflows Overview

1. **build-and-test.yml** - Runs tests and builds the application
2. **container-build.yml** - Builds and pushes Docker containers
3. **deploy-dev.yml** - Deploys to development environment
4. **terraform-deploy.yml** - Manages infrastructure with Terraform
5. **app-deploy.yml** - Handles application deployments to different environments

## Detailed Workflow Documentation

### 1. build-and-test.yml

**Purpose**: Runs unit tests and builds the application

**Triggers**:
- Push to main or test branches
- Pull requests to main or test branches

**Key Jobs**:
- Sets up Go environment
- Installs templ and TailwindCSS
- Runs all Go tests with PostgreSQL test database
- Executes build script

### 2. container-build.yml

**Purpose**: Builds and pushes Docker containers to ECR

**Triggers**:
- Push to dev branch with changes to:
  - app/ directory
  - Dockerfile
  - build.sh
  - docker-compose.yml

**Key Jobs**:
- Builds and tags Docker image
- Pushes to Amazon ECR
- Registers ECS task definition
- Updates ECS service

### 3. deploy-dev.yml

**Purpose**: Deploys to development environment

**Triggers**:
- Push to main or dev branches
- Manual trigger via workflow_dispatch

**Key Jobs**:
- Builds and pushes Docker image to ECR
- Registers new task definition
- Deploys to ECS with force-new-deployment
- Outputs ALB DNS name

### 4. terraform-deploy.yml

**Purpose**: Manages infrastructure with Terraform

**Triggers**:
- Push to main or dev branches with changes to infra/terraform/**

**Key Jobs**:
- Terraform init, plan, and apply
- Auto-applies on dev branch
- Requires manual approval for prod

### 5. app-deploy.yml

**Purpose**: Handles application deployments

**Triggers**:
- Push to main, dev, or test branches with changes to:
  - app/ directory
  - Dockerfile
  - go.mod

**Key Jobs**:
- Test environment: Runs tests and deploys to test cluster
- Dev/Prod: Builds and deploys to respective environments
- Uses different ECS clusters based on branch

## Common Configuration

**Required Secrets**:
- AWS_ACCESS_KEY_ID
- AWS_SECRET_ACCESS_KEY

**Environment Variables**:
- AWS_REGION: us-east-1
- ECR_REPOSITORY: transgo-dev
- ECS cluster/service names vary by environment

## Workflow Dependencies

1. build-and-test.yml runs first on code changes
2. container-build.yml creates Docker images
3. deploy-dev.yml/app-deploy.yml handle deployments
4. terraform-deploy.yml manages infrastructure
