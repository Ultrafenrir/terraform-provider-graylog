# Production Examples

This directory contains production-ready Terraform configurations demonstrating best practices for deploying and managing Graylog with this provider.

## Examples Overview

### 1. LDAP User Sync with RBAC
**File:** `ldap-sync-rbac.tf`

Complete workflow for syncing LDAP users to Graylog with role-based stream permissions.

**Features:**
- Automated user provisioning from LDAP groups
- Role-based access control (RBAC)
- Per-stream permissions for different teams
- Multi-group support with role accumulation

**Use case:** Organizations using LDAP/Active Directory for centralized user management.

**Deploy:**
```bash
cd examples/production

# Configure variables
export TF_VAR_graylog_url="https://graylog.example.com/api"
export TF_VAR_graylog_admin_password="your-password"
export TF_VAR_ldap_bind_password="ldap-password"

# Initialize and apply
terraform init
terraform plan -target=module.ldap_sync  # Review changes
terraform apply -target=module.ldap_sync -auto-approve
```

**Automation:** Set up scheduled sync (cron/CI) to run daily:
```bash
0 2 * * * cd /path/to/terraform && terraform apply -auto-approve
```

---

### 2. Backup and Restore Strategy
**File:** `backup-and-restore.tf`

Complete backup strategy using OpenSearch snapshots with S3 and filesystem repositories.

**Features:**
- Primary S3 bucket with lifecycle policies (hot/warm/cold/archive)
- DR region replication for disaster recovery
- IAM roles and policies for secure access
- Multiple snapshot repositories (S3, filesystem)
- Retention policies aligned with compliance requirements

**Use case:** Production environments requiring data backup and disaster recovery.

**Deploy:**
```bash
# Set AWS credentials
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."

# Configure variables
export TF_VAR_opensearch_instance_role_name="opensearch-ec2-role"
export TF_VAR_environment="production"

# Apply
terraform init
terraform apply -target=aws_s3_bucket.graylog_backups_primary
terraform apply  # Full deployment
```

**Create snapshot:**
```bash
# Via OpenSearch API
REPO="graylog-primary-backups"
SNAPSHOT="snapshot_$(date +%Y%m%d_%H%M%S)"

curl -X PUT "https://opensearch:9200/_snapshot/${REPO}/${SNAPSHOT}" \
  -H 'Content-Type: application/json' \
  -d '{
    "indices": "graylog_*",
    "ignore_unavailable": true,
    "include_global_state": false
  }'
```

**Automated snapshots:** See `docs/guides/stream-backups.md` for Lambda/CronJob examples.

---

## Architecture Patterns

### Pattern A: LDAP-Driven Multi-Tenancy

```
LDAP Groups (devops, security, engineering)
    ↓
Terraform (reads groups via graylog_ldap_group_members)
    ↓
Graylog Users + Roles (auto-created)
    ↓
Stream Permissions (role-based access)
```

**Result:** Each team sees only their streams; fully automated user lifecycle.

---

### Pattern B: Multi-Region Backup with DR Failover

```
Primary Region (us-east-1)
    ↓
OpenSearch Snapshots → S3 Primary Bucket
    ↓ (S3 Replication)
DR Region (us-west-2)
    ↓
S3 DR Bucket (read-only repository)
```

**Result:** RTO 4 hours, RPO 24 hours; automated cross-region replication.

---

## Deployment Workflow

### Initial Setup

1. **Clone and configure:**
   ```bash
   git clone https://github.com/your-org/graylog-terraform.git
   cd graylog-terraform/production
   cp terraform.tfvars.example terraform.tfvars
   # Edit terraform.tfvars with your values
   ```

2. **Initialize:**
   ```bash
   terraform init
   terraform workspace new production
   ```

3. **Plan and review:**
   ```bash
   terraform plan -out=plan.tfplan
   # Review plan carefully
   ```

4. **Apply:**
   ```bash
   terraform apply plan.tfplan
   ```

### Ongoing Operations

**Daily LDAP sync:**
```bash
# Cron job
0 2 * * * cd /path/to/terraform && terraform apply -auto-approve -target=graylog_user.ldap_users
```

**Weekly backup verification:**
```bash
# Test restore from latest snapshot
./scripts/test-restore.sh
```

**Monthly disaster recovery drill:**
```bash
# Fail over to DR region and restore data
./scripts/dr-failover.sh
```

---

## Best Practices

### 1. **State Management**

**Remote backend (S3 + DynamoDB):**
```hcl
terraform {
  backend "s3" {
    bucket         = "terraform-state-graylog-prod"
    key            = "graylog/production/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-locks"
  }
}
```

**Workspace isolation:**
```bash
terraform workspace new staging
terraform workspace new production
terraform workspace select production
```

### 2. **Secret Management**

**Use environment variables:**
```bash
export TF_VAR_graylog_admin_password="$(vault read -field=password secret/graylog/admin)"
export TF_VAR_ldap_bind_password="$(vault read -field=password secret/ldap/bind)"
```

**Or integrate with HashiCorp Vault:**
```hcl
data "vault_generic_secret" "graylog_admin" {
  path = "secret/graylog/admin"
}

provider "graylog" {
  password = data.vault_generic_secret.graylog_admin.data["password"]
}
```

### 3. **Change Management**

**Pull request workflow:**
1. Create feature branch
2. Make changes and commit
3. Open PR with `terraform plan` output
4. Peer review plan
5. Merge and apply via CI/CD

**Example GitHub Actions:**
```yaml
name: Terraform Apply
on:
  push:
    branches: [main]

jobs:
  apply:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: hashicorp/setup-terraform@v3
      - name: Terraform Apply
        env:
          TF_VAR_graylog_admin_password: ${{ secrets.GRAYLOG_PASSWORD }}
        run: |
          terraform init
          terraform apply -auto-approve
```

### 4. **Monitoring & Alerting**

**Monitor Terraform state drift:**
```bash
# Weekly cron job to detect drift
terraform plan -detailed-exitcode
if [ $? -eq 2 ]; then
  echo "Drift detected!" | mail -s "Terraform Drift Alert" ops@example.com
fi
```

**Monitor backup health:**
```bash
# Check latest snapshot status
curl -s "https://opensearch:9200/_snapshot/graylog-primary-backups/_all" | \
  jq '.snapshots | sort_by(.start_time_in_millis) | last | select(.state != "SUCCESS") | .state' | \
  xargs -I {} echo "Snapshot failed: {}" | mail -s "Backup Alert" ops@example.com
```

### 5. **Documentation**

**Maintain runbooks:**
- `docs/runbooks/disaster-recovery.md` — DR failover procedure
- `docs/runbooks/user-offboarding.md` — Remove LDAP users
- `docs/runbooks/backup-restore.md` — Restore from snapshot

**Document changes:**
```bash
# Git commit messages
git commit -m "Add security team to LDAP sync

- Added security-team LDAP group
- Created SecurityRole with audit log access
- Granted read/edit permissions on security_events stream

Ref: JIRA-123"
```

---

## Troubleshooting

### LDAP Sync Issues

**Problem:** Users not syncing from LDAP.

**Debug:**
```bash
# Test LDAP connectivity
ldapsearch -x -H ldap://ldap.example.com:389 \
  -D "cn=readonly,dc=example,dc=com" -w password \
  -b "dc=example,dc=com" "(cn=devops)"

# Check Terraform output
terraform apply
terraform output ldap_users_synced
```

**Solution:** Verify LDAP credentials, group name, and network connectivity.

---

### Backup Failures

**Problem:** Snapshots fail with "AccessDenied" error.

**Debug:**
```bash
# Check IAM permissions
aws iam get-role-policy --role-name opensearch-instance-role \
  --policy-name opensearch-snapshot-access

# Test S3 access from OpenSearch node
aws s3 ls s3://graylog-backups-production --region us-east-1
```

**Solution:** Ensure IAM role has correct S3 permissions (see `backup-and-restore.tf`).

---

## Cost Optimization

### S3 Storage Costs

**Lifecycle transitions:**
- Hot (7 days): S3 Standard (~$0.023/GB/month)
- Warm (30 days): S3 IA (~$0.0125/GB/month)
- Cold (90 days): Glacier (~$0.004/GB/month)
- Archive (365 days): Deep Archive (~$0.00099/GB/month)

**Example:** 1TB of snapshots over 1 year:
- Without lifecycle: 1TB × $0.023 × 12 = **$276/year**
- With lifecycle: ~**$80/year** (67% savings)

### OpenSearch Snapshot Bandwidth

**Rate limiting:**
```hcl
s3_settings {
  max_snapshot_bytes_per_sec = "100mb"  # Limit bandwidth to reduce data transfer costs
}
```

---

## Security Checklist

- [ ] Use IAM roles instead of access keys (where possible)
- [ ] Enable S3 bucket encryption (AES256 or KMS)
- [ ] Enable S3 bucket versioning (protect against deletes)
- [ ] Block public S3 access
- [ ] Rotate Graylog API credentials regularly
- [ ] Use LDAPS or StartTLS for LDAP connections
- [ ] Store Terraform state in encrypted S3 backend
- [ ] Enable state locking with DynamoDB
- [ ] Review IAM policies (least privilege principle)
- [ ] Enable CloudTrail for S3 bucket access logs
- [ ] Set up alerting for backup failures

---

## Related Documentation

- [LDAP User Sync Guide](../../docs/guides/ldap-user-sync.md)
- [Stream Backups Guide](../../docs/guides/stream-backups.md)
- [Troubleshooting Guide](../../docs/guides/troubleshooting.md)
- [Provider Documentation](https://registry.terraform.io/providers/Ultrafenrir/graylog/latest/docs)

---

## Support

For issues or questions:
- GitHub Issues: https://github.com/Ultrafenrir/terraform-provider-graylog/issues
- Terraform Registry: https://registry.terraform.io/providers/Ultrafenrir/graylog

---

**Last updated:** 2026-03-14
