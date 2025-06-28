# CI/CD Workflows Documentation

This directory contains GitHub Actions workflows that automate our build, test, and deployment processes.

## Current Workflow Structure

**ci-cd.yml** - Comprehensive CI/CD pipeline that handles all build, test, and deployment operations

## Workflow Overview

### ci-cd.yml

**Purpose**: Complete CI/CD pipeline for the TransGo application

**Triggers**:
- Push to any branch (`**`)

**Key Features**:
- **Smart AWS Detection**: Automatically detects AWS credentials and runs appropriate mode:
  - With AWS credentials: Full build, test, Docker build, push to ECR, Terraform deploy, ECS deployment
  - Without AWS credentials: Local test mode (build and test only)

**Pipeline Steps**:

1. **Setup Phase**:
   - Checkout code
   - Extract branch name for environment-specific deployments
   - Setup Go 1.24 with module caching
   - Install templ for template generation
   - Setup Node.js 18

2. **Build Phase**:
   - Create static CSS files (basic styling)
   - Generate Go templates from .templ files
   - Build Go application
   - Run all tests

3. **AWS Deployment Phase** (only if AWS credentials available):
   - Configure AWS credentials
   - Login to Amazon ECR
   - Build and push Docker image with proper tagging
   - Create branch-specific Terraform environment
   - Initialize and apply Terraform configuration
   - Deploy to ECS with service stability wait

**Environment Handling**:
- Branch names are sanitized (slashes converted to dashes)
- Each branch gets its own ECR repository: `transgo-{branch-name}`
- Each branch gets its own ECS cluster: `transgo-cluster-{branch-name}`
- Terraform environments are created dynamically from template

**Docker Image Tagging**:
- SHA tag: `{registry}/transgo-{branch}:{commit-sha}`
- Latest tag: `{registry}/transgo-{branch}:latest`

**Required Secrets**:
- `AWS_ACCESS_KEY_ID` (optional - enables AWS deployment)
- `AWS_SECRET_ACCESS_KEY` (optional - enables AWS deployment)

**Environment Variables**:
- `AWS_REGION`: us-east-1
- `BRANCH_NAME`: Automatically extracted from git ref

## Workflow Benefits

1. **Single Source of Truth**: One workflow handles all CI/CD operations
2. **Environment Flexibility**: Works locally without AWS credentials for testing
3. **Branch-based Deployments**: Each branch gets its own isolated environment
4. **Modern Actions**: Uses latest versions of all GitHub Actions
5. **Efficient Caching**: Go modules are cached for faster builds
6. **Error Handling**: Comprehensive error handling and conditional execution

## Migration Notes

This workflow replaces the following deprecated workflows:
- `build-and-test.yml` - Build and test functionality integrated
- `container-build.yml` - Docker build/push functionality integrated  
- `deploy-dev.yml` - Deployment functionality integrated
- `app-deploy.yml` - Application deployment functionality integrated
- `terraform-deploy.yml` - Terraform deployment functionality integrated
- All `reusable-*.yml` workflows - No longer needed with consolidated approach

The new approach provides better maintainability, reduces duplication, and ensures consistent behavior across all environments.
