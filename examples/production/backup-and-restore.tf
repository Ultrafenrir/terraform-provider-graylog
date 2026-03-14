# Complete Backup and Restore Setup for Graylog Streams
# This example demonstrates:
# - S3 snapshot repository configuration
# - Filesystem snapshot repository (for on-premise)
# - S3 bucket lifecycle policies
# - IAM permissions for OpenSearch
# - Multi-region backup strategy

terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = "~> 0.3"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "graylog" {
  url            = var.graylog_url
  opensearch_url = var.opensearch_url
  username       = var.graylog_admin_user
  password       = var.graylog_admin_password
}

provider "aws" {
  region = var.aws_region
}

# Variables
variable "graylog_url" {
  description = "Graylog API URL"
  default     = "https://graylog.example.com"
}

variable "opensearch_url" {
  description = "OpenSearch URL"
  default     = "https://opensearch.example.com:9200"
}

variable "graylog_admin_user" {
  default = "admin"
}

variable "graylog_admin_password" {
  sensitive = true
}

variable "aws_region" {
  description = "Primary AWS region for backups"
  default     = "us-east-1"
}

variable "dr_region" {
  description = "Disaster recovery region"
  default     = "us-west-2"
}

variable "environment" {
  description = "Environment name"
  default     = "production"
}

variable "opensearch_instance_role_name" {
  description = "IAM role name attached to OpenSearch instances"
  type        = string
}

# ========================================
# Primary Region: S3 Backup Configuration
# ========================================

# S3 bucket for primary backups
resource "aws_s3_bucket" "graylog_backups_primary" {
  bucket = "graylog-backups-${var.environment}-${var.aws_region}"

  tags = {
    Name        = "Graylog Backups (Primary)"
    Environment = var.environment
    Region      = var.aws_region
    ManagedBy   = "Terraform"
  }
}

# Enable versioning (protect against accidental deletes/overwrites)
resource "aws_s3_bucket_versioning" "backups_primary" {
  bucket = aws_s3_bucket.graylog_backups_primary.id

  versioning_configuration {
    status = "Enabled"
  }
}

# Server-side encryption
resource "aws_s3_bucket_server_side_encryption_configuration" "backups_primary" {
  bucket = aws_s3_bucket.graylog_backups_primary.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
      # Or use KMS for enhanced security:
      # sse_algorithm     = "aws:kms"
      # kms_master_key_id = aws_kms_key.graylog_backups.arn
    }
  }
}

# Lifecycle rules (data retention strategy)
resource "aws_s3_bucket_lifecycle_configuration" "backups_primary" {
  bucket = aws_s3_bucket.graylog_backups_primary.id

  # Hot snapshots (last 7 days)
  rule {
    id     = "hot_snapshots"
    status = "Enabled"

    filter {
      prefix = "snapshots/hot/"
    }

    expiration {
      days = 7
    }
  }

  # Warm snapshots (7-30 days)
  rule {
    id     = "warm_snapshots"
    status = "Enabled"

    filter {
      prefix = "snapshots/warm/"
    }

    transition {
      days          = 7
      storage_class = "STANDARD_IA"  # Infrequent Access
    }

    expiration {
      days = 30
    }
  }

  # Cold snapshots (30-90 days, archived)
  rule {
    id     = "cold_snapshots"
    status = "Enabled"

    filter {
      prefix = "snapshots/cold/"
    }

    transition {
      days          = 30
      storage_class = "GLACIER"
    }

    expiration {
      days = 90
    }
  }

  # Long-term archive (compliance: 1 year retention)
  rule {
    id     = "archive_snapshots"
    status = "Enabled"

    filter {
      prefix = "snapshots/archive/"
    }

    transition {
      days          = 90
      storage_class = "DEEP_ARCHIVE"
    }

    expiration {
      days = 365
    }
  }

  # Cleanup old versions
  rule {
    id     = "cleanup_old_versions"
    status = "Enabled"

    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }
}

# Block public access
resource "aws_s3_bucket_public_access_block" "backups_primary" {
  bucket = aws_s3_bucket.graylog_backups_primary.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# ========================================
# DR Region: S3 Backup Configuration
# ========================================

# DR bucket (disaster recovery)
resource "aws_s3_bucket" "graylog_backups_dr" {
  provider = aws.dr

  bucket = "graylog-backups-${var.environment}-${var.dr_region}"

  tags = {
    Name        = "Graylog Backups (DR)"
    Environment = var.environment
    Region      = var.dr_region
    ManagedBy   = "Terraform"
  }
}

provider "aws" {
  alias  = "dr"
  region = var.dr_region
}

# Enable versioning on DR bucket
resource "aws_s3_bucket_versioning" "backups_dr" {
  provider = aws.dr
  bucket   = aws_s3_bucket.graylog_backups_dr.id

  versioning_configuration {
    status = "Enabled"
  }
}

# Encryption for DR bucket
resource "aws_s3_bucket_server_side_encryption_configuration" "backups_dr" {
  provider = aws.dr
  bucket   = aws_s3_bucket.graylog_backups_dr.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# S3 Replication (primary → DR)
resource "aws_s3_bucket_replication_configuration" "backups_replication" {
  bucket = aws_s3_bucket.graylog_backups_primary.id
  role   = aws_iam_role.replication.arn

  rule {
    id     = "replicate_all"
    status = "Enabled"

    filter {
      prefix = "snapshots/"
    }

    destination {
      bucket        = aws_s3_bucket.graylog_backups_dr.arn
      storage_class = "STANDARD"

      # Replicate encryption
      encryption_configuration {
        replica_kms_key_id = null  # Use same encryption method
      }
    }

    delete_marker_replication {
      status = "Enabled"
    }
  }

  depends_on = [
    aws_s3_bucket_versioning.backups_primary,
    aws_s3_bucket_versioning.backups_dr
  ]
}

# IAM role for S3 replication
resource "aws_iam_role" "replication" {
  name = "graylog-s3-replication-${var.environment}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "s3.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_role_policy" "replication" {
  role = aws_iam_role.replication.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetReplicationConfiguration",
          "s3:ListBucket"
        ]
        Resource = aws_s3_bucket.graylog_backups_primary.arn
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObjectVersionForReplication",
          "s3:GetObjectVersionAcl"
        ]
        Resource = "${aws_s3_bucket.graylog_backups_primary.arn}/*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:ReplicateObject",
          "s3:ReplicateDelete"
        ]
        Resource = "${aws_s3_bucket.graylog_backups_dr.arn}/*"
      }
    ]
  })
}

# ========================================
# IAM Permissions for OpenSearch
# ========================================

# IAM policy for OpenSearch to write snapshots
resource "aws_iam_policy" "opensearch_s3_snapshots" {
  name        = "opensearch-snapshot-access-${var.environment}"
  description = "Allow OpenSearch to manage snapshots in S3"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucket",
          "s3:GetBucketLocation",
          "s3:ListBucketMultipartUploads",
          "s3:ListBucketVersions"
        ]
        Resource = [
          aws_s3_bucket.graylog_backups_primary.arn,
          aws_s3_bucket.graylog_backups_dr.arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:AbortMultipartUpload",
          "s3:ListMultipartUploadParts"
        ]
        Resource = [
          "${aws_s3_bucket.graylog_backups_primary.arn}/*",
          "${aws_s3_bucket.graylog_backups_dr.arn}/*"
        ]
      }
    ]
  })
}

# Attach policy to OpenSearch instance role
resource "aws_iam_role_policy_attachment" "opensearch_snapshots" {
  role       = var.opensearch_instance_role_name
  policy_arn = aws_iam_policy.opensearch_s3_snapshots.arn
}

# ========================================
# OpenSearch Snapshot Repositories
# ========================================

# Primary S3 repository
resource "graylog_opensearch_snapshot_repository" "primary_s3" {
  name = "graylog-primary-backups"
  type = "s3"

  s3_settings {
    bucket    = aws_s3_bucket.graylog_backups_primary.id
    region    = var.aws_region
    base_path = "snapshots/daily"

    # Use IAM role (recommended for EC2/ECS/EKS)
    # Omit access_key/secret_key when using instance roles

    compress                   = true
    server_side_encryption     = true
    max_snapshot_bytes_per_sec = "200mb"
    max_restore_bytes_per_sec  = "200mb"
    chunk_size                 = "500mb"
  }

  depends_on = [
    aws_iam_role_policy_attachment.opensearch_snapshots
  ]
}

# DR S3 repository (for restore in DR region)
resource "graylog_opensearch_snapshot_repository" "dr_s3" {
  name = "graylog-dr-backups"
  type = "s3"

  s3_settings {
    bucket    = aws_s3_bucket.graylog_backups_dr.id
    region    = var.dr_region
    base_path = "snapshots/daily"

    # Read-only mode (prevent accidental writes to DR)
    read_only = true

    compress                  = true
    max_restore_bytes_per_sec = "200mb"
  }

  depends_on = [
    aws_iam_role_policy_attachment.opensearch_snapshots
  ]
}

# Filesystem repository (on-premise or hybrid)
resource "graylog_opensearch_snapshot_repository" "filesystem" {
  name = "graylog-local-backups"
  type = "fs"

  fs_settings {
    location = "/mnt/opensearch/snapshots"  # Must be in path.repo

    compress                   = true
    max_snapshot_bytes_per_sec = "100mb"
    max_restore_bytes_per_sec  = "100mb"
  }
}

# ========================================
# Outputs
# ========================================

output "primary_bucket" {
  description = "Primary S3 backup bucket name"
  value       = aws_s3_bucket.graylog_backups_primary.id
}

output "dr_bucket" {
  description = "DR S3 backup bucket name"
  value       = aws_s3_bucket.graylog_backups_dr.id
}

output "snapshot_repositories" {
  description = "Configured snapshot repositories"
  value = {
    primary    = graylog_opensearch_snapshot_repository.primary_s3.name
    dr         = graylog_opensearch_snapshot_repository.dr_s3.name
    filesystem = graylog_opensearch_snapshot_repository.filesystem.name
  }
}

output "backup_strategy" {
  description = "Backup retention strategy (RTO/RPO)"
  value = {
    rto_hours = 4
    rpo_hours = 24
    retention = {
      hot     = "7 days (S3 Standard)"
      warm    = "30 days (S3 IA)"
      cold    = "90 days (Glacier)"
      archive = "365 days (Deep Archive)"
    }
    replication = "Primary (${var.aws_region}) → DR (${var.dr_region})"
  }
}

# ========================================
# Usage Instructions
# ========================================

# To create a snapshot manually:
# curl -X PUT "https://opensearch:9200/_snapshot/graylog-primary-backups/snapshot_$(date +%Y%m%d_%H%M%S)" \
#   -H 'Content-Type: application/json' \
#   -d '{"indices":"graylog_*","ignore_unavailable":true,"include_global_state":false}'

# To list snapshots:
# curl -X GET "https://opensearch:9200/_snapshot/graylog-primary-backups/_all?pretty"

# To restore from snapshot:
# curl -X POST "https://opensearch:9200/_snapshot/graylog-primary-backups/snapshot_NAME/_restore" \
#   -H 'Content-Type: application/json' \
#   -d '{"indices":"graylog_*","ignore_unavailable":true,"include_global_state":false}'

# For DR failover, use DR repository:
# curl -X POST "https://dr-opensearch:9200/_snapshot/graylog-dr-backups/snapshot_NAME/_restore" \
#   -H 'Content-Type: application/json' \
#   -d '{"indices":"graylog_*"}'
