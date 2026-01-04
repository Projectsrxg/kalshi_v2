# Kalshi Data Platform Infrastructure
#
# Architecture:
# - 3 gatherer instances (one per AZ) with local PostgreSQL + TimescaleDB
# - 1 deduplicator instance
# - 1 production RDS (TimescaleDB)
# - S3 bucket for archival exports
#
# See docs/kalshi-data/deployment/terraform.md for details.

terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

variable "aws_region" {
  description = "AWS region for deployment"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
}

# TODO: Define resources
# - VPC and networking
# - EC2 instances for gatherers
# - RDS for production database
# - S3 bucket for exports
# - Security groups
# - IAM roles
