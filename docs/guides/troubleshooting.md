---
page_title: "Troubleshooting Guide - Graylog Provider"
subcategory: "Guides"
description: |-
  Common issues, debugging techniques, and solutions for the Graylog Terraform Provider.
---

# Troubleshooting Guide

This guide covers common issues, error messages, and debugging techniques for the Graylog Terraform Provider.

## Table of Contents

- [Authentication Issues](#authentication-issues)
- [API Errors](#api-errors)
- [Version Compatibility](#version-compatibility)
- [Resource-Specific Issues](#resource-specific-issues)
- [Performance Issues](#performance-issues)
- [Debugging Techniques](#debugging-techniques)

## Authentication Issues

### Error: 401 Unauthorized

**Symptom:**
```
Error: API request failed: 401 Unauthorized
```

**Causes & Solutions:**

1. **Incorrect credentials:**
   ```hcl
   provider "graylog" {
     url      = "https://graylog.example.com/api"
     username = "admin"
     password = var.graylog_password  # Verify this value!
   }
   ```

   **Debug:**
   ```bash
   # Test credentials manually
   curl -u admin:password https://graylog.example.com/api/system
   ```

2. **Using wrong auth method:**
   ```hcl
   # If using API token, specify auth_method explicitly
   provider "graylog" {
     url         = "https://graylog.example.com/api"
     auth_method = "basic_token"
     api_token   = var.api_token
   }
   ```

3. **Legacy token format:**
   ```hcl
   # Old format (deprecated but still supported)
   provider "graylog" {
     url   = "https://graylog.example.com/api"
     token = base64encode("username:password")
   }
   ```

### Error: 403 Forbidden

**Symptom:**
```
Error: insufficient permissions to perform this action
```

**Cause:** User lacks required permissions.

**Solution:**
1. **Check user role:**
   ```bash
   curl -u admin:password https://graylog.example.com/api/users/username
   ```

2. **Grant Admin role** (for provider operations):
   ```hcl
   resource "graylog_user" "terraform" {
     username = "terraform-sa"
     # ...
     roles = ["Admin"]  # Required for most operations
   }
   ```

3. **Minimum permissions needed:**
   - Streams: `streams:read`, `streams:create`, `streams:edit`
   - Inputs: `inputs:read`, `inputs:create`, `inputs:edit`
   - Users/Roles: Requires `Admin` role

## API Errors

### Error: 404 Not Found (Resource)

**Symptom:**
```
Error: resource not found
```

**Causes:**

1. **Resource deleted outside Terraform:**
   ```bash
   # Refresh state to detect drift
   terraform refresh
   ```

2. **Wrong resource ID:**
   ```hcl
   # Verify ID format (UUID or 24-hex ObjectID)
   data "graylog_stream" "example" {
     id = "507f1f77bcf86cd799439011"  # Must exist in Graylog
   }
   ```

3. **Import failed:**
   ```bash
   # Re-import with correct ID
   terraform import graylog_stream.s 507f1f77bcf86cd799439011
   ```

### Error: 500 Internal Server Error

**Symptom:**
```
Error: API request failed: 500 Internal Server Error
{
  "type": "ApiError",
  "message": "..."
}
```

**Debugging:**

1. **Check Graylog server logs:**
   ```bash
   # Docker
   docker logs graylog

   # Systemd
   journalctl -u graylog-server -f
   ```

2. **Enable provider debug logging:**
   ```hcl
   provider "graylog" {
     # ...
     log_level = "DEBUG"
   }
   ```

   ```bash
   # Run Terraform with verbose logging
   TF_LOG=DEBUG terraform apply 2>&1 | tee debug.log
   ```

3. **Common causes:**
   - Invalid JSON in `config` fields (inputs, alerts)
   - Malformed pipeline source syntax
   - Index set configuration conflicts

### Error: Validation Errors

**Symptom:**
```
Error: validation failed
{
  "errors": {
    "field_name": ["error message"]
  }
}
```

**Solutions:**

1. **Input configuration validation:**
   ```hcl
   # Ensure all required fields are present
   resource "graylog_input" "kafka" {
     type  = "org.graylog2.inputs.kafka.KafkaInput"
     configuration = {
       # Required fields for Kafka input
       topic_filter              = "logs-.*"
       zookeeper                 = "zk.example.com:2181"
       fetch_min_bytes           = "5"
       # ... (check Graylog docs for required fields)
     }
   }
   ```

2. **Stream rule type validation:**
   ```hcl
   resource "graylog_stream" "example" {
     rule {
       field = "source"
       type  = 1  # Must be valid enum for your Graylog version
       value = "server1"
     }
   }
   ```

   **Valid rule types** (version-dependent):
   - `1` = exact match
   - `2` = greater than
   - `3` = regex
   - `5` = contains
   - See [Stream Rule Types](#stream-rule-types) section

## Version Compatibility

### Graylog 5.x / 6.x / 7.x Differences

**Issue:** Features available in one version but not others.

**Solution:** Use capability checks or version-specific configuration.

1. **Classic Dashboards** (availability varies):
   ```hcl
   # Some Graylog 5.x/6.x images don't support classic dashboards
   resource "graylog_dashboard" "example" {
     # May fail with "feature not available" on certain versions
     title       = "Example"
     description = "..."
   }
   ```

   **Workaround:** Check Graylog image/version or use Views instead.

2. **API prefix** (handled automatically):
   - Graylog 5.x: `http://host:9000/api/...`
   - Graylog 6.x/7.x: `http://host:9000/api/...` (same, but prefix required)

   Provider handles this automatically — just configure `url` consistently:
   ```hcl
   provider "graylog" {
     url = "http://host:9000"  # Provider adds /api as needed
   }
   ```

### Stream Rule Types by Version

Rule type enum values **vary by Graylog version**:

| Type | GL 5.x | GL 6.x | GL 7.x | Description |
|------|--------|--------|--------|-------------|
| Exact | 1 | 1 | 1 | Field exactly matches value |
| Greater | 2 | 2 | 2 | Numeric field > value |
| Regex | 3 | 3 | 3 | Field matches regex pattern |
| Smaller | 4 | 4 | 4 | Numeric field < value |
| Contains | 5 | 5 | 5 | Field contains substring |

**Always consult your Graylog version docs** for authoritative enum values.

## Resource-Specific Issues

### Streams

#### Issue: Rules not applying

**Symptom:** Stream created, but rules don't match messages.

**Debug:**
1. **Check rule syntax:**
   ```hcl
   resource "graylog_stream" "app_logs" {
     title = "Application Logs"

     rule {
       field    = "application"
       type     = 1  # Exact match
       value    = "myapp"
       inverted = false  # Ensure not inverted by mistake
     }
   }
   ```

2. **Test rule in Graylog UI:**
   - Navigate to stream → Rules
   - Verify rule is present
   - Check "Throughput / Metrics" for matches

3. **Common mistakes:**
   - Field name typo (`applicaton` vs `application`)
   - Wrong rule type (using regex type `3` with non-regex value)
   - Inverted logic

#### Issue: Stream permissions not working

**Symptom:** Users can't access stream despite `graylog_stream_permission`.

**Debug:**
```hcl
resource "graylog_stream_permission" "example" {
  role_name = "Reader"
  stream_id = graylog_stream.app_logs.id
  actions   = ["read"]
}
```

**Check:**
1. **Role exists:**
   ```bash
   curl -u admin:password https://graylog.example.com/api/roles/Reader
   ```

2. **User has role assigned:**
   ```hcl
   resource "graylog_user" "alice" {
     username = "alice"
     roles    = ["Reader"]  # Must include role!
   }
   ```

3. **Permission format:**
   ```bash
   # Verify permission is applied to role
   curl -u admin:password https://graylog.example.com/api/roles/Reader | \
     jq '.permissions[] | select(contains("streams:read"))'
   ```

### Inputs

#### Issue: Extractor parsing errors

**Symptom:** Extractors fail to parse, logs show errors.

**Cause:** Invalid extractor JSON format.

**Solution:**
```hcl
resource "graylog_input" "syslog" {
  # ...

  extractors = jsonencode([
    {
      title       = "Parse timestamp"
      type        = "GROK"
      cursor_strategy = "COPY"
      source_field    = "message"
      target_field    = "timestamp"
      extractor_config = {
        grok_pattern = "%{TIMESTAMP_ISO8601:timestamp}"
      }
      condition_type  = "NONE"
      condition_value = ""
      order          = 0
    }
  ])
}
```

**Debug:**
1. **Validate JSON:**
   ```bash
   echo 'your_json' | jq .
   ```

2. **Check Graylog extractor docs** for required fields per extractor type.

3. **Test in Graylog UI first**, then export JSON and adapt to Terraform.

### Alerts (Event Definitions)

#### Issue: Typed threshold block fails

**Symptom:**
```
Error: validation failed on threshold configuration
```

**Solution:** Verify all required fields:
```hcl
resource "graylog_alert" "high_error_rate" {
  title = "High Error Rate"

  threshold {
    threshold_type = "more"
    threshold      = 100
    field          = "error_count"

    # Required fields
    time_range_type  = "range"
    time_range_value = 5  # minutes

    # Optional but recommended
    grace_ms      = 60000  # 1 minute grace period
    backlog_size  = 100    # Include 100 messages in alert
  }

  notification_ids = [graylog_event_notification.email.id]
}
```

**Fallback:** Use `config` escape hatch for complex cases:
```hcl
resource "graylog_alert" "custom" {
  title = "Custom Alert"

  config = jsonencode({
    type = "aggregation-v1"
    # ... full GL event definition JSON
  })
}
```

### OpenSearch Snapshot Repositories

#### Issue: "path.repo not configured"

**Error:**
```
location [/snapshots] doesn't match any of the locations specified by path.repo
```

**Solution:** Configure OpenSearch `opensearch.yml`:
```yaml
path.repo: ["/snapshots", "/mnt/backups"]
```

Restart OpenSearch after change.

#### Issue: S3 AccessDenied

**Error:**
```
Access Denied (Service: Amazon S3; Status Code: 403; Error Code: AccessDenied)
```

**Solution:**
1. **Verify IAM policy:**
   ```json
   {
     "Effect": "Allow",
     "Action": ["s3:ListBucket", "s3:GetBucketLocation"],
     "Resource": "arn:aws:s3:::graylog-backups"
   },
   {
     "Effect": "Allow",
     "Action": ["s3:PutObject", "s3:GetObject", "s3:DeleteObject"],
     "Resource": "arn:aws:s3:::graylog-backups/*"
   }
   ```

2. **Check OpenSearch instance role** (if using IAM roles)

3. **Test S3 access from OpenSearch node:**
   ```bash
   aws s3 ls s3://graylog-backups --region us-east-1
   ```

### LDAP Group Members

#### Issue: No members returned

**Symptom:**
```hcl
data "graylog_ldap_group_members" "devops" {
  # ... config
}

output "members" {
  value = data.graylog_ldap_group_members.devops.members
}
# Output: []
```

**Debug:**
1. **Test LDAP connectivity:**
   ```bash
   ldapsearch -x -H ldap://ldap.example.com:389 \
     -D "cn=admin,dc=example,dc=com" -w password \
     -b "dc=example,dc=com" "(cn=devops)"
   ```

2. **Check group filter:**
   ```hcl
   data "graylog_ldap_group_members" "devops" {
     # ...
     group_filter = "(cn=%s)"  # Default; adjust if needed
   }
   ```

3. **Verify member attribute:**
   ```hcl
   data "graylog_ldap_group_members" "devops" {
     # ...
     member_attr = "member"  # Default for groupOfNames
     # Use "memberUid" for posixGroup
   }
   ```

## Performance Issues

### Issue: Slow `terraform apply`

**Symptom:** Apply takes 5+ minutes for 10 resources.

**Causes:**

1. **Sequential API calls:**
   - Each resource = 1 API call
   - 100 streams = 100 sequential creates

   **Workaround:** None currently; bulk operations planned for future versions.

2. **API timeouts:**
   ```hcl
   provider "graylog" {
     timeout = "60s"  # Increase for slow API responses
   }
   ```

3. **Graylog under load:**
   - Check Graylog server CPU/memory
   - Review Graylog server logs for slow queries

### Issue: State refresh takes too long

**Symptom:** `terraform plan` hangs for minutes.

**Solution:**
1. **Reduce data source queries:**
   ```hcl
   # Avoid querying large lists on every plan
   # data "graylog_streams" "all" {}  # Returns 1000+ streams

   # Instead: use targeted lookups
   data "graylog_stream" "specific" {
     id = "507f1f77bcf86cd799439011"
   }
   ```

2. **Use `-refresh=false` for plan:**
   ```bash
   terraform plan -refresh=false  # Skip state refresh
   ```

## Debugging Techniques

### 1. Enable Debug Logging

**Provider-level:**
```hcl
provider "graylog" {
  log_level = "DEBUG"  # TRACE | DEBUG | INFO | WARN | ERROR
}
```

**Terraform-level:**
```bash
export TF_LOG=DEBUG
export TF_LOG_PATH=terraform-debug.log
terraform apply
```

**Filter to provider logs only:**
```bash
grep "graylog" terraform-debug.log
```

### 2. Inspect HTTP Requests

```bash
# Enable HTTP request/response logging
TF_LOG=TRACE terraform apply 2>&1 | grep -A 20 "HTTP Request"
```

**Example output:**
```
2026-03-14T10:00:00.000Z [DEBUG] HTTP Request: POST /api/streams
2026-03-14T10:00:00.000Z [DEBUG] Request Body: {"title":"Example","description":"..."}
2026-03-14T10:00:00.000Z [DEBUG] HTTP Response: 201 Created
```

### 3. Verify API Responses Manually

```bash
# Test same request outside Terraform
curl -X POST https://graylog.example.com/api/streams \
  -u admin:password \
  -H 'Content-Type: application/json' \
  -d '{"title":"Test Stream","description":"Manual test"}'
```

### 4. Check Terraform State

```bash
# Show resource state
terraform state show graylog_stream.example

# List all resources
terraform state list

# Inspect specific attribute
terraform state show graylog_stream.example | grep stream_id
```

### 5. Use `terraform console`

```bash
$ terraform console

> graylog_stream.example.id
"507f1f77bcf86cd799439011"

> data.graylog_ldap_group_members.devops.members
[
  {
    username = "alice"
    email = "alice@example.com"
    # ...
  }
]
```

### 6. Validate Configuration

```bash
# Check syntax
terraform validate

# Check formatting
terraform fmt -check

# Dry-run plan
terraform plan
```

## Common Error Messages

### "context deadline exceeded"

**Cause:** Request timeout.

**Solution:**
```hcl
provider "graylog" {
  timeout     = "120s"  # Increase from default 30s
  max_retries = 5       # Increase retries
}
```

### "connection refused"

**Cause:** Graylog API not reachable.

**Solution:**
1. **Verify URL:**
   ```bash
   curl https://graylog.example.com/api/system
   ```

2. **Check firewall/VPN:**
   ```bash
   nc -zv graylog.example.com 9000
   ```

3. **Verify Graylog is running:**
   ```bash
   docker ps | grep graylog
   # or
   systemctl status graylog-server
   ```

### "x509: certificate signed by unknown authority"

**Cause:** Self-signed TLS certificate.

**Solution (dev/test only):**
```hcl
provider "graylog" {
  insecure_skip_verify = true
}
```

**Production solution:**
```hcl
provider "graylog" {
  ca_bundle = "/path/to/ca-bundle.crt"
}
```

### "field is required but not set"

**Cause:** Missing required argument.

**Solution:** Check resource docs for required fields:
```bash
terraform-docs providers Ultrafenrir/graylog
# or visit: https://registry.terraform.io/providers/Ultrafenrir/graylog/latest/docs
```

## Getting Help

### 1. Collect Debug Information

Before opening an issue:

```bash
# Run with debug logging
TF_LOG=DEBUG terraform apply 2>&1 | tee debug.log

# Sanitize sensitive data
sed -i 's/password.*/password REDACTED/g' debug.log

# Include:
# - Terraform version: terraform version
# - Provider version: from terraform.lock.hcl
# - Graylog version: curl http://graylog/api/system
# - Relevant configuration (sanitized)
```

### 2. GitHub Issues

Open issue at: https://github.com/Ultrafenrir/terraform-provider-graylog/issues

**Include:**
- Provider version
- Graylog version
- Expected vs actual behavior
- Minimal reproducible configuration
- Sanitized debug logs

### 3. Community Resources

- Terraform Registry: https://registry.terraform.io/providers/Ultrafenrir/graylog
- Graylog Community: https://community.graylog.org/
- Terraform Community: https://discuss.hashicorp.com/c/terraform-providers

## Environment Variables Reference

All provider configuration attributes support environment variables:

```bash
# Provider basics
export GRAYLOG_URL="https://graylog.example.com/api"
export GRAYLOG_AUTH_METHOD="basic_userpass"
export GRAYLOG_USERNAME="admin"
export GRAYLOG_PASSWORD="password"

# Alternative auth
export GRAYLOG_API_TOKEN="your-api-token"
export GRAYLOG_BEARER_TOKEN="your-bearer-token"

# TLS/HTTP
export GRAYLOG_INSECURE="1"  # Skip TLS verify (dev only)
export GRAYLOG_CA_BUNDLE="/path/to/ca.crt"
export GRAYLOG_TIMEOUT="60s"
export GRAYLOG_MAX_RETRIES="5"

# Debugging
export GRAYLOG_LOG_LEVEL="DEBUG"

# OpenSearch
export OPENSEARCH_URL="https://opensearch.example.com:9200"
export OPENSEARCH_INSECURE="1"
```

Use in CI/CD:
```yaml
# GitHub Actions
env:
  GRAYLOG_URL: ${{ secrets.GRAYLOG_URL }}
  GRAYLOG_USERNAME: ${{ secrets.GRAYLOG_USER }}
  GRAYLOG_PASSWORD: ${{ secrets.GRAYLOG_PASS }}
```

## Summary

Most issues fall into these categories:

1. **Authentication** — verify credentials, check roles
2. **API errors** — enable debug logging, check Graylog logs
3. **Version compatibility** — consult version-specific docs
4. **Configuration errors** — validate JSON, check required fields
5. **Performance** — increase timeouts, optimize queries

**Pro tip:** Always test API requests manually with `curl` first to isolate provider vs API issues.
