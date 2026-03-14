---
page_title: "Graylog Stream Backups Guide - Graylog Provider"
subcategory: "Guides"
description: |-
  Complete guide for backing up Graylog stream data using OpenSearch snapshots (Graylog OSS backups).
---

# Graylog Stream Backups with OpenSearch Snapshots

This guide explains how to implement **Graylog stream backups** for Graylog OSS using OpenSearch snapshot repositories. Since Graylog OSS does not include a built-in "backup streams" feature, you can use this provider to configure and automate backups of the indices that store your stream data.

**Keywords:** graylog stream backups, graylog backups, graylog opensearch snapshots, graylog data backup, graylog disaster recovery.

## What Gets Backed Up?

Graylog **Streams** route messages into **Index Sets**, which are stored in **OpenSearch/Elasticsearch indices**. When you back up indices via OpenSearch snapshots, you're backing up the actual log data processed by your Streams.

**Scope of backups:**
- ✅ Log messages (stored in indices)
- ✅ Index mappings and settings
- ❌ Graylog configuration (streams, inputs, pipelines) — use Terraform state for this
- ❌ User accounts, roles, permissions — manage via Terraform resources

## Architecture

```
Graylog Streams → Index Sets → OpenSearch Indices
                                      ↓
                          OpenSearch Snapshot Repository
                          (Filesystem or S3-compatible)
                                      ↓
                              Snapshot Storage
                              (Backups retention)
```

## Prerequisites

### 1. OpenSearch Configuration

**For Filesystem Repositories:**
- OpenSearch must have `path.repo` configured in `opensearch.yml`:
  ```yaml
  path.repo: ["/usr/share/opensearch/snapshots"]
  ```
- Directory must be writable by OpenSearch process
- Mount directory to persistent storage (NFS, EBS volume, etc.)

**For S3 Repositories:**
- OpenSearch must have `repository-s3` plugin installed:
  ```bash
  bin/opensearch-plugin install repository-s3
  ```
- S3 bucket created with appropriate lifecycle policies
- IAM role/credentials with S3 write permissions

### 2. Provider Configuration

Configure provider with both Graylog and OpenSearch URLs:

```hcl
provider "graylog" {
  url            = "https://graylog.example.com/api"
  opensearch_url = "https://opensearch.example.com:9200"

  # Optional: if OpenSearch uses self-signed cert
  opensearch_insecure = true
}
```

## Step 1: Create Snapshot Repository

### Option A: Filesystem Repository (Simple, Single-Node)

```hcl
resource "graylog_opensearch_snapshot_repository" "local_backups" {
  name = "graylog_backups"
  type = "fs"

  fs_settings {
    location = "/snapshots/graylog"
    compress = true

    # Rate limiting (optional)
    max_snapshot_bytes_per_sec = "100mb"
    max_restore_bytes_per_sec  = "100mb"

    # Chunk size for large files (optional)
    chunk_size = "100mb"
  }
}
```

### Option B: S3 Repository (Production, Multi-Node)

```hcl
resource "graylog_opensearch_snapshot_repository" "s3_backups" {
  name = "graylog_s3_backups"
  type = "s3"

  s3_settings {
    bucket    = "graylog-backups-prod"
    region    = "us-east-1"
    base_path = "opensearch/snapshots"  # prefix inside bucket

    # Compression
    compress = true

    # Server-side encryption (optional)
    server_side_encryption = true

    # Performance tuning
    max_snapshot_bytes_per_sec = "200mb"
    max_restore_bytes_per_sec  = "200mb"
    chunk_size                 = "500mb"
  }
}
```

### Option C: MinIO (S3-Compatible, On-Premise)

```hcl
resource "graylog_opensearch_snapshot_repository" "minio_backups" {
  name = "graylog_minio"
  type = "s3"

  s3_settings {
    bucket            = "graylog-snapshots"
    endpoint          = "https://minio.example.com:9000"
    protocol          = "https"
    path_style_access = true  # Required for MinIO

    # MinIO credentials (use variables in production!)
    access_key = var.minio_access_key
    secret_key = var.minio_secret_key

    base_path = "snapshots"
    compress  = true
  }
}
```

## Step 2: Verify Repository

After creating the repository, verify it's accessible:

```bash
# Check repository exists
curl -X GET "https://opensearch.example.com:9200/_snapshot/graylog_backups"

# Verify repository health
curl -X POST "https://opensearch.example.com:9200/_snapshot/graylog_backups/_verify"
```

Expected response:
```json
{
  "nodes": {
    "node-id": {
      "name": "opensearch-node-1"
    }
  }
}
```

## Step 3: Create Snapshots

### Manual Snapshot (Testing)

```bash
# Create snapshot of all graylog indices
curl -X PUT "https://opensearch.example.com:9200/_snapshot/graylog_backups/snapshot_$(date +%Y%m%d_%H%M%S)" \
  -H 'Content-Type: application/json' \
  -d '{
    "indices": "graylog_*",
    "ignore_unavailable": true,
    "include_global_state": false,
    "metadata": {
      "taken_by": "terraform",
      "environment": "production"
    }
  }'
```

### Automated Snapshots via Terraform

Use `null_resource` with `local-exec` provisioner (demonstration only):

```hcl
resource "null_resource" "daily_snapshot" {
  # Trigger: run whenever repository changes
  triggers = {
    repo_id = graylog_opensearch_snapshot_repository.s3_backups.id
  }

  provisioner "local-exec" {
    command = <<-EOT
      curl -X PUT "${var.opensearch_url}/_snapshot/${graylog_opensearch_snapshot_repository.s3_backups.name}/snapshot_$(date +%Y%m%d_%H%M%S)" \
        -H 'Content-Type: application/json' \
        -d '{
          "indices": "graylog_*",
          "ignore_unavailable": true,
          "include_global_state": false
        }'
    EOT
  }
}
```

**⚠️ Production Warning:** `null_resource` with `local-exec` is **not recommended for production** backups:
- Runs only during `terraform apply`
- No scheduling or retry logic
- No monitoring or alerting

See [Automation Options](#automation-options) below for production-grade solutions.

## Step 4: Snapshot Lifecycle Management

### Option A: OpenSearch ISM (Index State Management)

Create ISM policy for automated snapshots:

```json
{
  "policy": {
    "description": "Snapshot old indices to S3",
    "default_state": "hot",
    "states": [
      {
        "name": "hot",
        "actions": [],
        "transitions": [
          {
            "state_name": "warm",
            "conditions": {
              "min_index_age": "7d"
            }
          }
        ]
      },
      {
        "name": "warm",
        "actions": [
          {
            "snapshot": {
              "repository": "graylog_s3_backups",
              "snapshot": "{{ctx.index}}_{{now/d}}"
            }
          }
        ],
        "transitions": [
          {
            "state_name": "delete",
            "conditions": {
              "min_index_age": "30d"
            }
          }
        ]
      },
      {
        "name": "delete",
        "actions": [
          {
            "delete": {}
          }
        ]
      }
    ]
  }
}
```

Apply policy:
```bash
curl -X PUT "https://opensearch.example.com:9200/_plugins/_ism/policies/graylog_snapshot_policy" \
  -H 'Content-Type: application/json' \
  -d @ism_policy.json
```

### Option B: S3 Lifecycle Rules

For S3-based repositories, configure bucket lifecycle:

```hcl
resource "aws_s3_bucket_lifecycle_configuration" "graylog_backups" {
  bucket = "graylog-backups-prod"

  rule {
    id     = "transition_old_snapshots"
    status = "Enabled"

    # Move to Glacier after 30 days
    transition {
      days          = 30
      storage_class = "GLACIER"
    }

    # Move to Deep Archive after 90 days
    transition {
      days          = 90
      storage_class = "DEEP_ARCHIVE"
    }

    # Delete after 365 days
    expiration {
      days = 365
    }
  }
}
```

## Step 5: Restore from Snapshot

### List Available Snapshots

```bash
curl -X GET "https://opensearch.example.com:9200/_snapshot/graylog_backups/_all?pretty"
```

### Restore Specific Snapshot

```bash
# Close indices before restore (if they exist)
curl -X POST "https://opensearch.example.com:9200/graylog_0/_close"

# Restore snapshot
curl -X POST "https://opensearch.example.com:9200/_snapshot/graylog_backups/snapshot_20260314_120000/_restore" \
  -H 'Content-Type: application/json' \
  -d '{
    "indices": "graylog_*",
    "ignore_unavailable": true,
    "include_global_state": false,
    "rename_pattern": "graylog_(.+)",
    "rename_replacement": "restored_graylog_$1"
  }'
```

### Monitor Restore Progress

```bash
curl -X GET "https://opensearch.example.com:9200/_snapshot/graylog_backups/snapshot_20260314_120000/_status?pretty"
```

## Complete Production Example

```hcl
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

# S3 bucket for backups
resource "aws_s3_bucket" "graylog_backups" {
  bucket = "graylog-backups-${var.environment}"

  tags = {
    Name        = "Graylog Backups"
    Environment = var.environment
    ManagedBy   = "Terraform"
  }
}

# Enable versioning (protect against accidental deletes)
resource "aws_s3_bucket_versioning" "backups" {
  bucket = aws_s3_bucket.graylog_backups.id

  versioning_configuration {
    status = "Enabled"
  }
}

# Server-side encryption
resource "aws_s3_bucket_server_side_encryption_configuration" "backups" {
  bucket = aws_s3_bucket.graylog_backups.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Lifecycle rules
resource "aws_s3_bucket_lifecycle_configuration" "backups" {
  bucket = aws_s3_bucket.graylog_backups.id

  rule {
    id     = "snapshot_retention"
    status = "Enabled"

    transition {
      days          = 30
      storage_class = "GLACIER"
    }

    transition {
      days          = 90
      storage_class = "DEEP_ARCHIVE"
    }

    expiration {
      days = 365
    }
  }
}

# IAM policy for OpenSearch to access S3
resource "aws_iam_policy" "opensearch_s3" {
  name        = "opensearch-snapshot-access-${var.environment}"
  description = "Allow OpenSearch to write snapshots to S3"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucket",
          "s3:GetBucketLocation"
        ]
        Resource = aws_s3_bucket.graylog_backups.arn
      },
      {
        Effect = "Allow"
        Action = [
          "s3:PutObject",
          "s3:GetObject",
          "s3:DeleteObject"
        ]
        Resource = "${aws_s3_bucket.graylog_backups.arn}/*"
      }
    ]
  })
}

# Attach policy to OpenSearch instance role (assume role exists)
resource "aws_iam_role_policy_attachment" "opensearch_s3" {
  role       = var.opensearch_instance_role_name
  policy_arn = aws_iam_policy.opensearch_s3.arn
}

# Create snapshot repository
resource "graylog_opensearch_snapshot_repository" "production" {
  name = "graylog-production-backups"
  type = "s3"

  s3_settings {
    bucket    = aws_s3_bucket.graylog_backups.id
    region    = var.aws_region
    base_path = "snapshots/${var.environment}"

    # Use IAM role (recommended) instead of access keys
    # If OpenSearch runs on EC2 with instance role, omit access_key/secret_key

    compress                   = true
    server_side_encryption     = true
    max_snapshot_bytes_per_sec = "200mb"
    max_restore_bytes_per_sec  = "200mb"
  }

  depends_on = [
    aws_iam_role_policy_attachment.opensearch_s3
  ]
}

# Outputs
output "snapshot_repository_name" {
  value = graylog_opensearch_snapshot_repository.production.name
}

output "s3_bucket_name" {
  value = aws_s3_bucket.graylog_backups.id
}
```

## Automation Options

### Option 1: Cron Job (Simple)

```bash
#!/bin/bash
# /etc/cron.daily/graylog-snapshot.sh

REPO="graylog_s3_backups"
SNAPSHOT="snapshot_$(date +%Y%m%d_%H%M%S)"
OPENSEARCH_URL="https://opensearch.example.com:9200"

curl -X PUT "${OPENSEARCH_URL}/_snapshot/${REPO}/${SNAPSHOT}" \
  -H 'Content-Type: application/json' \
  -d '{
    "indices": "graylog_*",
    "ignore_unavailable": true,
    "include_global_state": false
  }' || echo "Snapshot failed" | mail -s "Graylog Backup Alert" admin@example.com
```

### Option 2: AWS Lambda (Serverless)

```python
import boto3
import requests
from datetime import datetime

def lambda_handler(event, context):
    opensearch_url = os.environ['OPENSEARCH_URL']
    repo_name = os.environ['SNAPSHOT_REPO']
    snapshot_name = f"snapshot_{datetime.now().strftime('%Y%m%d_%H%M%S')}"

    url = f"{opensearch_url}/_snapshot/{repo_name}/{snapshot_name}"
    payload = {
        "indices": "graylog_*",
        "ignore_unavailable": True,
        "include_global_state": False
    }

    response = requests.put(url, json=payload)

    if response.status_code != 200:
        raise Exception(f"Snapshot failed: {response.text}")

    return {
        'statusCode': 200,
        'body': f'Snapshot {snapshot_name} created successfully'
    }
```

Scheduled via EventBridge:
```hcl
resource "aws_cloudwatch_event_rule" "daily_snapshot" {
  name                = "graylog-daily-snapshot"
  schedule_expression = "cron(0 2 * * ? *)"  # 2 AM daily
}

resource "aws_cloudwatch_event_target" "snapshot_lambda" {
  rule      = aws_cloudwatch_event_rule.daily_snapshot.name
  target_id = "SnapshotLambda"
  arn       = aws_lambda_function.graylog_snapshot.arn
}
```

### Option 3: Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: graylog-snapshot
spec:
  schedule: "0 2 * * *"  # 2 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: snapshot
            image: curlimages/curl:latest
            env:
            - name: OPENSEARCH_URL
              value: "http://opensearch:9200"
            - name: REPO_NAME
              value: "graylog_backups"
            command:
            - /bin/sh
            - -c
            - |
              SNAPSHOT="snapshot_$(date +%Y%m%d_%H%M%S)"
              curl -X PUT "${OPENSEARCH_URL}/_snapshot/${REPO_NAME}/${SNAPSHOT}" \
                -H 'Content-Type: application/json' \
                -d '{"indices":"graylog_*","ignore_unavailable":true,"include_global_state":false}'
          restartPolicy: OnFailure
```

## Monitoring & Alerting

### Check Snapshot Status

```bash
# List recent snapshots
curl -X GET "https://opensearch.example.com:9200/_snapshot/graylog_backups/_all?pretty" | \
  jq '.snapshots[] | {name: .snapshot, state: .state, start_time: .start_time}'

# Check failed snapshots
curl -X GET "https://opensearch.example.com:9200/_snapshot/graylog_backups/_all?pretty" | \
  jq '.snapshots[] | select(.state == "FAILED")'
```

### Prometheus Metrics (OpenSearch Exporter)

Monitor snapshot health via Prometheus:
```yaml
# Alert rule
- alert: GraylogSnapshotFailed
  expr: opensearch_snapshot_stats_number_of_failed_snapshots > 0
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "Graylog snapshot failed"
    description: "{{ $value }} snapshots failed in repository {{ $labels.repository }}"
```

## Best Practices

### 1. **Repository Redundancy**
```hcl
# Primary: S3 in us-east-1
resource "graylog_opensearch_snapshot_repository" "primary" {
  name = "primary_backups"
  type = "s3"
  s3_settings {
    bucket = "graylog-backups-primary"
    region = "us-east-1"
  }
}

# Secondary: S3 in us-west-2 (disaster recovery)
resource "graylog_opensearch_snapshot_repository" "secondary" {
  name = "dr_backups"
  type = "s3"
  s3_settings {
    bucket = "graylog-backups-dr"
    region = "us-west-2"
  }
}
```

### 2. **Retention Policies**
- **Hot data:** Last 7 days in OpenSearch
- **Warm data:** 7-30 days in S3 Standard
- **Cold data:** 30-90 days in S3 Glacier
- **Archive:** 90-365 days in S3 Deep Archive
- **Delete:** After 365 days (compliance dependent)

### 3. **Test Restores Regularly**
```bash
#!/bin/bash
# Monthly restore test (run via cron)

REPO="graylog_backups"
LATEST_SNAPSHOT=$(curl -s "https://opensearch:9200/_snapshot/${REPO}/_all" | \
  jq -r '.snapshots | sort_by(.start_time_in_millis) | last | .snapshot')

# Restore to test index
curl -X POST "https://opensearch:9200/_snapshot/${REPO}/${LATEST_SNAPSHOT}/_restore" \
  -H 'Content-Type: application/json' \
  -d '{
    "indices": "graylog_0",
    "rename_pattern": "graylog_(.+)",
    "rename_replacement": "restore_test_$1"
  }'

# Verify data
COUNT=$(curl -s "https://opensearch:9200/restore_test_0/_count" | jq '.count')
if [ "$COUNT" -gt 0 ]; then
  echo "Restore test PASSED: $COUNT documents restored"
else
  echo "Restore test FAILED" | mail -s "Backup Alert" admin@example.com
fi

# Cleanup test index
curl -X DELETE "https://opensearch:9200/restore_test_*"
```

### 4. **Security**
- Use IAM roles instead of access keys where possible
- Enable S3 bucket versioning (protect against accidental deletes)
- Encrypt snapshots at rest (S3 SSE or KMS)
- Restrict OpenSearch API access (firewall/VPC)

## Disaster Recovery Runbook

### Scenario: Complete Data Loss

1. **Verify backup repository is accessible:**
   ```bash
   curl -X GET "https://new-opensearch:9200/_snapshot/graylog_backups"
   ```

2. **Register repository on new cluster** (if needed):
   ```bash
   # Re-run Terraform to recreate repository resource
   terraform apply -target=graylog_opensearch_snapshot_repository.production
   ```

3. **List available snapshots:**
   ```bash
   curl -X GET "https://new-opensearch:9200/_snapshot/graylog_backups/_all?pretty"
   ```

4. **Restore latest snapshot:**
   ```bash
   LATEST_SNAPSHOT="snapshot_20260314_020000"

   curl -X POST "https://new-opensearch:9200/_snapshot/graylog_backups/${LATEST_SNAPSHOT}/_restore" \
     -H 'Content-Type: application/json' \
     -d '{
       "indices": "graylog_*",
       "ignore_unavailable": true,
       "include_global_state": false
     }'
   ```

5. **Monitor restore progress:**
   ```bash
   curl -X GET "https://new-opensearch:9200/_cat/recovery?v&h=index,stage,type,time"
   ```

6. **Verify data integrity:**
   ```bash
   # Check document counts
   curl -X GET "https://new-opensearch:9200/graylog_*/_count?pretty"

   # Spot check recent messages
   curl -X GET "https://new-opensearch:9200/graylog_*/_search?size=10&sort=timestamp:desc"
   ```

7. **Restore Graylog configuration from Terraform:**
   ```bash
   terraform apply  # Recreates streams, inputs, dashboards, users, etc.
   ```

**RTO (Recovery Time Objective):** 2-4 hours (depends on snapshot size)
**RPO (Recovery Point Objective):** Last successful snapshot (24 hours if daily)

## Troubleshooting

### Error: "path.repo not configured"

```
[graylog_backups] [fs] location [/snapshots] doesn't match any of the locations specified by path.repo
```

**Solution:** Add `path.repo` to `opensearch.yml` and restart:
```yaml
path.repo: ["/snapshots", "/mnt/backups"]
```

### Error: "AccessDenied" (S3)

**Solution:** Verify IAM policy and OpenSearch instance role:
```bash
# Test S3 access from OpenSearch node
aws s3 ls s3://graylog-backups-prod --region us-east-1
```

### Slow Snapshot Performance

**Solution:** Increase rate limits:
```hcl
resource "graylog_opensearch_snapshot_repository" "fast" {
  # ...
  s3_settings {
    max_snapshot_bytes_per_sec = "500mb"  # Increase from default 40mb
    chunk_size                 = "1gb"    # Larger chunks for big files
  }
}
```

### Snapshot Stuck in "IN_PROGRESS"

```bash
# Check snapshot status
curl -X GET "https://opensearch:9200/_snapshot/graylog_backups/stuck_snapshot/_status"

# If truly stuck, delete and retry
curl -X DELETE "https://opensearch:9200/_snapshot/graylog_backups/stuck_snapshot"
```

## Summary

This workflow enables **Graylog stream backups for OSS** users:

- ✅ Automated backups via OpenSearch snapshots
- ✅ Multiple storage backends (S3, filesystem, MinIO)
- ✅ Lifecycle management (retention policies)
- ✅ Disaster recovery procedures
- ✅ Infrastructure as Code (version controlled, reproducible)

**Next steps:**
- Set up automated snapshot scheduling (cron/Lambda/K8s CronJob)
- Implement monitoring and alerting
- Test restore procedures quarterly
- Document RTO/RPO requirements
