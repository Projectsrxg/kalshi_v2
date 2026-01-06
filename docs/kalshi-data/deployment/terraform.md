# Terraform

Infrastructure-as-Code for the Kalshi Data Platform.

---

## Module Structure

```
deploy/terraform/
├── main.tf           # VPC, subnets, internet gateway
├── ec2.tf            # Gatherer and deduplicator instances
├── rds.tf            # Production RDS
├── s3.tf             # Parquet export bucket
├── security.tf       # Security groups
├── iam.tf            # IAM roles and instance profiles
├── variables.tf      # Input variables
├── outputs.tf        # Output values
└── terraform.tfvars  # Variable values (gitignored)
```

---

## variables.tf

```hcl
variable "project" {
  description = "Project name"
  type        = string
  default     = "kalshi-data"
}

variable "environment" {
  description = "Environment (prod, staging, dev)"
  type        = string
  default     = "prod"
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "vpc_cidr" {
  description = "VPC CIDR block"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "Availability zones"
  type        = list(string)
  default     = ["us-east-1a", "us-east-1b", "us-east-1c"]
}

variable "gatherer_instance_type" {
  description = "EC2 instance type for gatherers"
  type        = string
  default     = "t4g.2xlarge"
}

variable "deduplicator_instance_type" {
  description = "EC2 instance type for deduplicator"
  type        = string
  default     = "t4g.xlarge"
}

variable "rds_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t4g.large"
}

variable "rds_allocated_storage" {
  description = "RDS storage in GB"
  type        = number
  default     = 500
}

variable "admin_ip" {
  description = "Admin IP for SSH access"
  type        = string
}

variable "db_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}
```

---

## main.tf

```hcl
terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket = "kalshi-terraform-state"
    key    = "kalshi-data/terraform.tfstate"
    region = "us-east-1"
  }
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = var.project
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

# VPC
resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "${var.project}-vpc"
  }
}

# Internet Gateway
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${var.project}-igw"
  }
}

# Public Subnets
resource "aws_subnet" "public" {
  count = length(var.availability_zones)

  vpc_id                  = aws_vpc.main.id
  cidr_block              = cidrsubnet(var.vpc_cidr, 8, count.index + 1)
  availability_zone       = var.availability_zones[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.project}-public-${var.availability_zones[count.index]}"
    Type = "public"
  }
}

# Private Subnets
resource "aws_subnet" "private" {
  count = length(var.availability_zones)

  vpc_id            = aws_vpc.main.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index + 101)
  availability_zone = var.availability_zones[count.index]

  tags = {
    Name = "${var.project}-private-${var.availability_zones[count.index]}"
    Type = "private"
  }
}

# Public Route Table
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "${var.project}-public-rt"
  }
}

# Associate public subnets with public route table
resource "aws_route_table_association" "public" {
  count = length(aws_subnet.public)

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# VPC Endpoint for S3
resource "aws_vpc_endpoint" "s3" {
  vpc_id       = aws_vpc.main.id
  service_name = "com.amazonaws.${var.region}.s3"

  tags = {
    Name = "${var.project}-s3-endpoint"
  }
}
```

---

## security.tf

```hcl
# Gatherer Security Group
resource "aws_security_group" "gatherer" {
  name        = "${var.project}-gatherer-sg"
  description = "Security group for gatherer instances"
  vpc_id      = aws_vpc.main.id

  # SSH from admin
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["${var.admin_ip}/32"]
  }

  # TimescaleDB from deduplicator
  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.deduplicator.id]
  }

  # PostgreSQL from deduplicator
  ingress {
    from_port       = 5433
    to_port         = 5433
    protocol        = "tcp"
    security_groups = [aws_security_group.deduplicator.id]
  }

  # Health check from deduplicator
  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.deduplicator.id]
  }

  # Prometheus metrics from VPC
  ingress {
    from_port   = 9090
    to_port     = 9090
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  # Outbound to Kalshi API
  egress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project}-gatherer-sg"
  }
}

# Deduplicator Security Group
resource "aws_security_group" "deduplicator" {
  name        = "${var.project}-deduplicator-sg"
  description = "Security group for deduplicator instance"
  vpc_id      = aws_vpc.main.id

  # SSH from admin
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["${var.admin_ip}/32"]
  }

  # Health check from VPC
  ingress {
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  # Prometheus metrics from VPC
  ingress {
    from_port   = 9090
    to_port     = 9090
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  # Outbound to gatherers
  egress {
    from_port       = 5432
    to_port         = 5433
    protocol        = "tcp"
    security_groups = [aws_security_group.gatherer.id]
  }

  # Outbound to RDS
  egress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.rds.id]
  }

  # Outbound to S3 (via VPC endpoint)
  egress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project}-deduplicator-sg"
  }
}

# RDS Security Group
resource "aws_security_group" "rds" {
  name        = "${var.project}-rds-sg"
  description = "Security group for production RDS"
  vpc_id      = aws_vpc.main.id

  # PostgreSQL from deduplicator
  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.deduplicator.id]
  }

  tags = {
    Name = "${var.project}-rds-sg"
  }
}
```

---

## iam.tf

```hcl
# Gatherer IAM Role
resource "aws_iam_role" "gatherer" {
  name = "${var.project}-gatherer-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role_policy" "gatherer" {
  name = "${var.project}-gatherer-policy"
  role = aws_iam_role.gatherer.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:*:*:log-group:/kalshi/*"
      },
      {
        Effect   = "Allow"
        Action   = ["cloudwatch:PutMetricData"]
        Resource = "*"
        Condition = {
          StringEquals = {
            "cloudwatch:namespace" = "Kalshi"
          }
        }
      }
    ]
  })
}

resource "aws_iam_instance_profile" "gatherer" {
  name = "${var.project}-gatherer-profile"
  role = aws_iam_role.gatherer.name
}

# Deduplicator IAM Role
resource "aws_iam_role" "deduplicator" {
  name = "${var.project}-deduplicator-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role_policy" "deduplicator" {
  name = "${var.project}-deduplicator-policy"
  role = aws_iam_role.deduplicator.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:PutObject",
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.data.arn,
          "${aws_s3_bucket.data.arn}/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:*:*:log-group:/kalshi/*"
      },
      {
        Effect   = "Allow"
        Action   = ["cloudwatch:PutMetricData"]
        Resource = "*"
        Condition = {
          StringEquals = {
            "cloudwatch:namespace" = "Kalshi"
          }
        }
      }
    ]
  })
}

resource "aws_iam_instance_profile" "deduplicator" {
  name = "${var.project}-deduplicator-profile"
  role = aws_iam_role.deduplicator.name
}
```

---

## ec2.tf

```hcl
# Latest Amazon Linux 2023 ARM64 AMI
data "aws_ami" "amazon_linux" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-arm64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# Gatherer Instances
resource "aws_instance" "gatherer" {
  count = length(var.availability_zones)

  ami                    = data.aws_ami.amazon_linux.id
  instance_type          = var.gatherer_instance_type
  subnet_id              = aws_subnet.public[count.index].id
  vpc_security_group_ids = [aws_security_group.gatherer.id]
  iam_instance_profile   = aws_iam_instance_profile.gatherer.name

  root_block_device {
    volume_type = "gp3"
    volume_size = 200
    iops        = 3000
    throughput  = 125
  }

  # SECURITY: Passwords fetched from Secrets Manager at runtime, not in user_data
  user_data = base64encode(templatefile("${path.module}/scripts/gatherer-init.sh", {
    gatherer_id     = count.index + 1
    secrets_prefix  = "${var.project}/prod"
    aws_region      = var.aws_region
  }))

  tags = {
    Name = "${var.project}-gatherer-${count.index + 1}"
    Role = "gatherer"
  }
}

# Deduplicator Instance
resource "aws_instance" "deduplicator" {
  ami                    = data.aws_ami.amazon_linux.id
  instance_type          = var.deduplicator_instance_type
  subnet_id              = aws_subnet.public[2].id  # us-east-1c
  vpc_security_group_ids = [aws_security_group.deduplicator.id]
  iam_instance_profile   = aws_iam_instance_profile.deduplicator.name

  root_block_device {
    volume_type = "gp3"
    volume_size = 50
    iops        = 3000
    throughput  = 125
  }

  # SECURITY: Passwords fetched from Secrets Manager at runtime, not in user_data
  user_data = base64encode(templatefile("${path.module}/scripts/deduplicator-init.sh", {
    gatherer_ips   = [for i in aws_instance.gatherer : i.private_ip]
    rds_endpoint   = aws_db_instance.production.endpoint
    secrets_prefix = "${var.project}/prod"
    aws_region     = var.aws_region
  }))

  tags = {
    Name = "${var.project}-deduplicator"
    Role = "deduplicator"
  }

  depends_on = [aws_instance.gatherer]
}
```

---

## rds.tf

```hcl
# DB Subnet Group
resource "aws_db_subnet_group" "main" {
  name       = "${var.project}-db-subnet-group"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "${var.project}-db-subnet-group"
  }
}

# Production RDS Instance
resource "aws_db_instance" "production" {
  identifier = "${var.project}-prod"

  engine         = "postgres"
  engine_version = "16"
  instance_class = var.rds_instance_class

  allocated_storage     = var.rds_allocated_storage
  storage_type          = "gp3"
  storage_encrypted     = true
  iops                  = 3000
  storage_throughput    = 125

  db_name  = "kalshi_prod"
  username = "kalshi"
  password = var.db_password

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false
  multi_az               = false

  backup_retention_period = 7
  backup_window           = "03:00-04:00"
  maintenance_window      = "Sun:04:00-Sun:05:00"

  skip_final_snapshot = false
  final_snapshot_identifier = "${var.project}-prod-final"

  tags = {
    Name = "${var.project}-prod"
  }
}
```

---

## s3.tf

```hcl
resource "aws_s3_bucket" "data" {
  bucket = "${var.project}-${var.environment}"

  tags = {
    Name = "${var.project}-${var.environment}"
  }
}

resource "aws_s3_bucket_versioning" "data" {
  bucket = aws_s3_bucket.data.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "data" {
  bucket = aws_s3_bucket.data.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "data" {
  bucket = aws_s3_bucket.data.id

  rule {
    id     = "parquet-lifecycle"
    status = "Enabled"

    filter {
      prefix = "parquet/"
    }

    transition {
      days          = 0
      storage_class = "INTELLIGENT_TIERING"
    }

    transition {
      days          = 365
      storage_class = "GLACIER"
    }
  }

  rule {
    id     = "backups-lifecycle"
    status = "Enabled"

    filter {
      prefix = "backups/"
    }

    transition {
      days          = 30
      storage_class = "GLACIER"
    }
  }
}
```

---

## outputs.tf

```hcl
output "vpc_id" {
  description = "VPC ID"
  value       = aws_vpc.main.id
}

output "gatherer_ips" {
  description = "Gatherer public IPs"
  value       = aws_instance.gatherer[*].public_ip
}

output "gatherer_private_ips" {
  description = "Gatherer private IPs"
  value       = aws_instance.gatherer[*].private_ip
}

output "deduplicator_ip" {
  description = "Deduplicator public IP"
  value       = aws_instance.deduplicator.public_ip
}

output "rds_endpoint" {
  description = "RDS endpoint"
  value       = aws_db_instance.production.endpoint
}

output "s3_bucket" {
  description = "S3 bucket name"
  value       = aws_s3_bucket.data.id
}
```

---

## secrets.tf

```hcl
# Secrets Manager for database credentials
# IMPORTANT: Create these secrets BEFORE running terraform apply

resource "aws_secretsmanager_secret" "api_key" {
  name        = "${var.project}/prod/api-key"
  description = "Kalshi API key"
}

resource "aws_secretsmanager_secret" "timescaledb_password" {
  name        = "${var.project}/prod/timescaledb-password"
  description = "TimescaleDB password for gatherer"
}

resource "aws_secretsmanager_secret" "postgresql_password" {
  name        = "${var.project}/prod/postgresql-password"
  description = "PostgreSQL password for gatherer"
}

resource "aws_secretsmanager_secret" "rds_password" {
  name        = "${var.project}/prod/rds-password"
  description = "Production RDS password"
}

resource "aws_secretsmanager_secret" "gatherer_reader_password" {
  name        = "${var.project}/prod/gatherer-reader-password"
  description = "Password for deduplicator to read from gatherers"
}

# IAM policy to read secrets
resource "aws_iam_policy" "secrets_read" {
  name        = "${var.project}-secrets-read"
  description = "Allow reading secrets from Secrets Manager"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = [
          "arn:aws:secretsmanager:${var.aws_region}:*:secret:${var.project}/prod/*"
        ]
      }
    ]
  })
}

# Attach to gatherer role
resource "aws_iam_role_policy_attachment" "gatherer_secrets" {
  role       = aws_iam_role.gatherer.name
  policy_arn = aws_iam_policy.secrets_read.arn
}

# Attach to deduplicator role
resource "aws_iam_role_policy_attachment" "deduplicator_secrets" {
  role       = aws_iam_role.deduplicator.name
  policy_arn = aws_iam_policy.secrets_read.arn
}
```

---

## scripts/gatherer-init.sh

```bash
#!/bin/bash
# Gatherer initialization script
# Variables injected by Terraform: gatherer_id, secrets_prefix, aws_region

set -e

GATHERER_ID="${gatherer_id}"
SECRETS_PREFIX="${secrets_prefix}"
AWS_REGION="${aws_region}"

echo "Initializing gatherer-$GATHERER_ID..."

# Install dependencies
dnf install -y postgresql16-server timescaledb jq

# Fetch secrets from Secrets Manager
get_secret() {
    aws secretsmanager get-secret-value \
        --region "$AWS_REGION" \
        --secret-id "$SECRETS_PREFIX/$1" \
        --query 'SecretString' \
        --output text
}

export KALSHI_API_KEY=$(get_secret "api-key")
export TIMESCALEDB_PASSWORD=$(get_secret "timescaledb-password")
export POSTGRESQL_PASSWORD=$(get_secret "postgresql-password")

# Initialize PostgreSQL
postgresql-setup --initdb
systemctl start postgresql

# Create databases
sudo -u postgres psql -c "CREATE DATABASE kalshi_ts;"
sudo -u postgres psql -c "CREATE DATABASE kalshi_meta;"
sudo -u postgres psql -d kalshi_ts -c "CREATE EXTENSION IF NOT EXISTS timescaledb;"

# Create users with passwords from Secrets Manager
sudo -u postgres psql -c "CREATE USER gatherer WITH PASSWORD '$TIMESCALEDB_PASSWORD';"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE kalshi_ts TO gatherer;"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE kalshi_meta TO gatherer;"

# Create dedup_reader for deduplicator access
READER_PASSWORD=$(get_secret "gatherer-reader-password")
sudo -u postgres psql -c "CREATE USER dedup_reader WITH PASSWORD '$READER_PASSWORD';"
sudo -u postgres psql -c "GRANT CONNECT ON DATABASE kalshi_ts TO dedup_reader;"
sudo -u postgres psql -c "GRANT CONNECT ON DATABASE kalshi_meta TO dedup_reader;"

# Configure pg_hba.conf for remote deduplicator access
cat >> /var/lib/pgsql/data/pg_hba.conf <<EOF
hostssl kalshi_ts       dedup_reader    10.0.0.0/8    scram-sha-256
hostssl kalshi_meta     dedup_reader    10.0.0.0/8    scram-sha-256
EOF

# Configure PostgreSQL to listen on all interfaces
echo "listen_addresses = '*'" >> /var/lib/pgsql/data/postgresql.conf

systemctl restart postgresql

# Write gatherer config (passwords come from environment)
cat > /etc/kalshi/gatherer.yaml <<EOF
gatherer_id: "gatherer-$GATHERER_ID"

api:
  base_url: "https://api.elections.kalshi.com/trade-api/v2"
  ws_url: "wss://api.elections.kalshi.com/trade-api/ws/v2"
  api_key: "\${KALSHI_API_KEY}"

timescaledb:
  host: "localhost"
  port: 5432
  database: "kalshi_ts"
  user: "gatherer"
  password: "\${TIMESCALEDB_PASSWORD}"
  ssl_mode: "prefer"

postgresql:
  host: "localhost"
  port: 5432
  database: "kalshi_meta"
  user: "gatherer"
  password: "\${POSTGRESQL_PASSWORD}"
  ssl_mode: "prefer"

logging:
  level: "info"
  format: "json"
EOF

# Run migrations
/opt/kalshi/gatherer migrate --config /etc/kalshi/gatherer.yaml

# Create systemd environment file for secrets
cat > /etc/kalshi/gatherer.env <<EOF
KALSHI_API_KEY=$KALSHI_API_KEY
TIMESCALEDB_PASSWORD=$TIMESCALEDB_PASSWORD
POSTGRESQL_PASSWORD=$POSTGRESQL_PASSWORD
EOF
chmod 600 /etc/kalshi/gatherer.env

# Start gatherer service
systemctl enable gatherer
systemctl start gatherer

echo "Gatherer-$GATHERER_ID initialization complete"
```

---

## scripts/deduplicator-init.sh

```bash
#!/bin/bash
# Deduplicator initialization script
# Variables injected by Terraform: gatherer_ips, rds_endpoint, secrets_prefix, aws_region

set -e

GATHERER_IPS="${gatherer_ips}"
RDS_ENDPOINT="${rds_endpoint}"
SECRETS_PREFIX="${secrets_prefix}"
AWS_REGION="${aws_region}"

echo "Initializing deduplicator..."

# Install dependencies
dnf install -y jq postgresql16

# Fetch secrets from Secrets Manager
get_secret() {
    aws secretsmanager get-secret-value \
        --region "$AWS_REGION" \
        --secret-id "$SECRETS_PREFIX/$1" \
        --query 'SecretString' \
        --output text
}

export RDS_PASSWORD=$(get_secret "rds-password")
export GATHERER_READER_PASSWORD=$(get_secret "gatherer-reader-password")

# Download RDS CA certificate
wget https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem \
    -O /etc/kalshi/rds-ca-bundle.pem

# Wait for RDS to be available
until pg_isready -h "$RDS_ENDPOINT" -p 5432; do
    echo "Waiting for RDS..."
    sleep 5
done

# Wait for at least one gatherer to be healthy
GATHERER_READY=false
for i in {1..60}; do
    for g in $GATHERER_IPS; do
        if curl -s "http://$g:8080/health" | grep -q '"status":"healthy"'; then
            GATHERER_READY=true
            break 2
        fi
    done
    echo "Waiting for gatherers... ($i/60)"
    sleep 5
done

if [ "$GATHERER_READY" = false ]; then
    echo "ERROR: No gatherers available after 5 minutes"
    exit 1
fi

# Write deduplicator config
cat > /etc/kalshi/deduplicator.yaml <<EOF
deduplicator_id: "deduplicator-1"

gatherers:
$(for ip in $GATHERER_IPS; do
cat <<GATHERER
  - id: "gatherer-$(echo $ip | cut -d. -f4)"
    host: "$ip"
    timescaledb:
      port: 5432
      database: "kalshi_ts"
      user: "dedup_reader"
      password: "\${GATHERER_READER_PASSWORD}"
      ssl_mode: "require"
    postgresql:
      port: 5432
      database: "kalshi_meta"
      user: "dedup_reader"
      password: "\${GATHERER_READER_PASSWORD}"
      ssl_mode: "require"
GATHERER
done)

production:
  host: "$RDS_ENDPOINT"
  port: 5432
  database: "kalshi_prod"
  user: "deduplicator"
  password: "\${RDS_PASSWORD}"
  ssl_mode: "verify-full"
  ssl_root_cert: "/etc/kalshi/rds-ca-bundle.pem"

s3:
  enabled: true
  bucket: "kalshi-data-archive"
  region: "$AWS_REGION"

logging:
  level: "info"
  format: "json"
EOF

# Create systemd environment file for secrets
cat > /etc/kalshi/deduplicator.env <<EOF
RDS_PASSWORD=$RDS_PASSWORD
GATHERER_READER_PASSWORD=$GATHERER_READER_PASSWORD
EOF
chmod 600 /etc/kalshi/deduplicator.env

# Start deduplicator service
systemctl enable deduplicator
systemctl start deduplicator

echo "Deduplicator initialization complete"
```

---

## Usage

```bash
# Initialize
cd deploy/terraform
terraform init

# Plan
terraform plan -var-file="prod.tfvars"

# Apply
terraform apply -var-file="prod.tfvars"

# Destroy (careful!)
terraform destroy -var-file="prod.tfvars"
```

### prod.tfvars

```hcl
project     = "kalshi-data"
environment = "prod"
aws_region  = "us-east-1"
admin_ip    = "1.2.3.4"  # Your IP for SSH access

# NOTE: Database passwords are NOT in tfvars
# They are stored in AWS Secrets Manager and fetched at runtime
# See secrets.tf for secret definitions
```

### Pre-Deployment: Create Secrets

Before running `terraform apply`, create the required secrets:

```bash
# Create secrets with initial values
aws secretsmanager create-secret \
  --name "kalshi-data/prod/api-key" \
  --secret-string "your-kalshi-api-key"

aws secretsmanager create-secret \
  --name "kalshi-data/prod/timescaledb-password" \
  --secret-string "$(openssl rand -base64 32)"

aws secretsmanager create-secret \
  --name "kalshi-data/prod/postgresql-password" \
  --secret-string "$(openssl rand -base64 32)"

aws secretsmanager create-secret \
  --name "kalshi-data/prod/rds-password" \
  --secret-string "$(openssl rand -base64 32)"

aws secretsmanager create-secret \
  --name "kalshi-data/prod/gatherer-reader-password" \
  --secret-string "$(openssl rand -base64 32)"
```
