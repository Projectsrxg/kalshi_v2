# Terraform Infrastructure

Infrastructure-as-Code for Kalshi Data Platform on AWS.

## Prerequisites

- Terraform >= 1.0
- AWS CLI configured
- Appropriate IAM permissions

## Usage

```bash
# Initialize
terraform init

# Plan
terraform plan -var-file=prod.tfvars

# Apply
terraform apply -var-file=prod.tfvars
```

## Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `aws_region` | AWS region | `us-east-1` |
| `environment` | Environment name | - |

## Resources Created

- VPC with public/private subnets across 3 AZs
- EC2 instances for gatherers (one per AZ)
- EC2 instance for deduplicator
- RDS instance (TimescaleDB) for production database
- S3 bucket for data archival
- Security groups
- IAM roles and policies

## State Management

Use remote state backend (S3 + DynamoDB) for production:

```hcl
terraform {
  backend "s3" {
    bucket         = "kalshi-terraform-state"
    key            = "kalshi-data/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "terraform-locks"
  }
}
```
