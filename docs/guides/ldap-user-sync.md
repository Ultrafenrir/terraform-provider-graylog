---
page_title: "LDAP User Sync Guide - Graylog Provider"
subcategory: "Guides"
description: |-
  Complete workflow for syncing LDAP users to Graylog with role-based stream permissions (Graylog OSS LDAP integration).
---

# LDAP User Sync with Stream Permissions

This guide demonstrates how to implement **Graylog LDAP integration** for Graylog OSS using Terraform. Since Graylog OSS does not include built-in LDAP user synchronization, you can use this provider to:

1. Read LDAP group members
2. Create/update Graylog users automatically
3. Assign roles based on LDAP group membership
4. Grant stream-level permissions to those roles

**Keywords:** graylog ldap integration, sync ldap groups to graylog, graylog ldap groups sync, graylog user sync ldap.

## Prerequisites

- LDAP/AD server accessible from Terraform execution environment
- Graylog instance with API access
- LDAP group(s) containing users to sync (e.g., `devops`, `security-team`)
- Graylog roles already created (or create them via `graylog_role` resource)

## Architecture

```
LDAP Directory (groups: devops, security-team)
    ↓ (read-only query)
Terraform data source: graylog_ldap_group_members
    ↓ (for_each loop)
Create graylog_user resources
    ↓
Assign roles (Reader, PowerUser, etc.)
    ↓
Grant stream permissions via graylog_stream_permission
```

## Step 1: Read LDAP Group Members

```hcl
# Read members of LDAP group "devops"
data "graylog_ldap_group_members" "devops" {
  url           = "ldap://ldap.example.com:389"
  bind_dn       = "cn=readonly,dc=example,dc=com"
  bind_password = var.ldap_bind_password  # Store in Terraform variables or Vault
  base_dn       = "dc=example,dc=com"
  group_name    = "devops"
}

# Optional: Read multiple groups
data "graylog_ldap_group_members" "security" {
  url           = "ldap://ldap.example.com:389"
  bind_dn       = "cn=readonly,dc=example,dc=com"
  bind_password = var.ldap_bind_password
  base_dn       = "dc=example,dc=com"
  group_name    = "security-team"
}

# Debug: output member lists
output "devops_users" {
  value = [for m in data.graylog_ldap_group_members.devops.members : m.username]
}
```

## Step 2: Create Graylog Users from LDAP

```hcl
# Create users for DevOps group members
resource "graylog_user" "devops_users" {
  for_each = { for m in data.graylog_ldap_group_members.devops.members : m.username => m }

  username  = each.key
  email     = each.value.email
  full_name = coalesce(each.value.display_name, each.key)

  # Security best practice: do not store passwords in Terraform state
  # Users will authenticate via LDAP/SSO in Graylog
  set_password = false

  # Assign roles based on group membership
  roles = ["Reader", "DevOpsRole"]

  # Optional: timezone, session timeout
  timezone         = "UTC"
  session_timeout  = 3600000  # 1 hour in ms
}

# Create users for Security group
resource "graylog_user" "security_users" {
  for_each = { for m in data.graylog_ldap_group_members.security.members : m.username => m }

  username  = each.key
  email     = each.value.email
  full_name = coalesce(each.value.display_name, each.key)

  set_password = false
  roles        = ["Reader", "SecurityRole"]
}
```

## Step 3: Create Custom Roles (if needed)

```hcl
# Create a DevOps role with basic permissions
resource "graylog_role" "devops" {
  name        = "DevOpsRole"
  description = "DevOps team members (synced from LDAP)"

  # Base permissions (adjust as needed)
  permissions = [
    "dashboards:read",
    "inputs:read",
    "metrics:read",
    "streams:read",
  ]
}

# Create a Security role
resource "graylog_role" "security" {
  name        = "SecurityRole"
  description = "Security team members (synced from LDAP)"

  permissions = [
    "dashboards:read",
    "event_definitions:read",
    "event_notifications:read",
    "streams:read",
  ]
}
```

## Step 4: Grant Stream Permissions to Roles

```hcl
# DevOps team can read/edit application logs stream
resource "graylog_stream_permission" "devops_app_logs" {
  role_name = graylog_role.devops.name
  stream_id = graylog_stream.app_logs.id
  actions   = ["read", "edit"]
}

# Security team gets read-only access to security events stream
resource "graylog_stream_permission" "security_events" {
  role_name = graylog_role.security.name
  stream_id = graylog_stream.security_events.id
  actions   = ["read"]
}

# Security team can manage alerts
resource "graylog_stream_permission" "security_alerts" {
  role_name = graylog_role.security.name
  stream_id = graylog_stream.security_alerts.id
  actions   = ["read", "edit", "share"]
}
```

## Step 5: Example Streams Setup

```hcl
# Application logs stream (DevOps access)
resource "graylog_stream" "app_logs" {
  title       = "Application Logs"
  description = "Production application logs"
  disabled    = false

  rule {
    field = "application"
    type  = 1  # exact match
    value = "myapp"
  }
}

# Security events stream (Security team access)
resource "graylog_stream" "security_events" {
  title       = "Security Events"
  description = "Security-related events and alerts"
  disabled    = false

  rule {
    field = "event_type"
    type  = 1
    value = "security"
  }
}
```

## Complete Example: Multi-Group Sync

```hcl
terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = "~> 0.3"
    }
  }
}

provider "graylog" {
  url      = "https://graylog.example.com/api"
  username = var.graylog_admin_user
  password = var.graylog_admin_password
}

# Variables
variable "ldap_url" {
  default = "ldap://ldap.example.com:389"
}

variable "ldap_bind_dn" {
  default = "cn=readonly,dc=example,dc=com"
}

variable "ldap_bind_password" {
  sensitive = true
}

variable "ldap_base_dn" {
  default = "dc=example,dc=com"
}

# LDAP groups to sync
locals {
  ldap_groups = {
    devops = {
      group_name = "devops"
      roles      = ["Reader", "DevOpsRole"]
      streams = {
        app_logs = ["read", "edit"]
      }
    }
    security = {
      group_name = "security-team"
      roles      = ["Reader", "SecurityRole"]
      streams = {
        security_events = ["read", "edit", "share"]
      }
    }
  }
}

# Read all LDAP groups
data "graylog_ldap_group_members" "groups" {
  for_each = local.ldap_groups

  url           = var.ldap_url
  bind_dn       = var.ldap_bind_dn
  bind_password = var.ldap_bind_password
  base_dn       = var.ldap_base_dn
  group_name    = each.value.group_name
}

# Flatten all users from all groups
locals {
  all_ldap_users = merge([
    for group_key, group_config in local.ldap_groups : {
      for member in data.graylog_ldap_group_members.groups[group_key].members :
      member.username => {
        username     = member.username
        email        = member.email
        display_name = member.display_name
        roles        = group_config.roles
      }
    }
  ]...)
}

# Create users (deduplicated by username)
resource "graylog_user" "ldap_synced" {
  for_each = local.all_ldap_users

  username     = each.value.username
  email        = each.value.email
  full_name    = coalesce(each.value.display_name, each.value.username)
  set_password = false
  roles        = each.value.roles
}

# Create roles
resource "graylog_role" "team_roles" {
  for_each = {
    DevOpsRole   = ["dashboards:read", "inputs:read", "streams:read"]
    SecurityRole = ["event_definitions:read", "streams:read"]
  }

  name        = each.key
  description = "Managed by Terraform (LDAP sync)"
  permissions = each.value
}

# Create streams
resource "graylog_stream" "team_streams" {
  for_each = {
    app_logs        = { title = "Application Logs", field = "application", value = "myapp" }
    security_events = { title = "Security Events", field = "event_type", value = "security" }
  }

  title       = each.value.title
  description = "Managed by Terraform"
  disabled    = false

  rule {
    field = each.value.field
    type  = 1
    value = each.value.value
  }
}

# Grant permissions
resource "graylog_stream_permission" "devops_app_logs" {
  role_name = "DevOpsRole"
  stream_id = graylog_stream.team_streams["app_logs"].id
  actions   = ["read", "edit"]
}

resource "graylog_stream_permission" "security_events" {
  role_name = "SecurityRole"
  stream_id = graylog_stream.team_streams["security_events"].id
  actions   = ["read", "edit", "share"]
}
```

## Best Practices

### 1. **Password Management**
- Never store LDAP bind passwords in plaintext
- Use Terraform variables with sensitive flag
- Or integrate with HashiCorp Vault / AWS Secrets Manager

### 2. **User Lifecycle**
- Run `terraform apply` on schedule (cron/CI) to sync new users
- **Important:** This provider does NOT automatically delete users removed from LDAP
- Implement custom cleanup script or accept manual user removal

### 3. **Role Assignment Strategy**
```hcl
# Option A: One role per LDAP group
locals {
  user_roles = {
    for username, user in local.all_ldap_users :
    username => distinct(flatten([user.roles]))
  }
}

# Option B: Accumulate roles from multiple groups
# (if user is in both devops and security groups, they get both roles)
```

### 4. **Large Groups Handling**
- LDAP queries may time out for groups with 1000+ members
- Consider filtering or paginating (future enhancement)
- Monitor Terraform execution time

### 5. **LDAP Connection Security**
```hcl
# Production: use LDAPS or StartTLS
data "graylog_ldap_group_members" "secure" {
  url      = "ldaps://ldap.example.com:636"
  # or
  url      = "ldap://ldap.example.com:389"
  starttls = true

  # Only for dev/test:
  # insecure = true              # alias of insecure_skip_verify for self-signed certs
  # insecure_skip_verify = true  # legacy name
}
```

## Troubleshooting

### Users not syncing

1. **Verify LDAP connectivity:**
```bash
ldapsearch -x -H ldap://ldap.example.com:389 \
  -D "cn=readonly,dc=example,dc=com" -w PASSWORD \
  -b "dc=example,dc=com" "(cn=devops)"
```

2. **Check Terraform output:**
```bash
terraform apply
# Look for errors in data.graylog_ldap_group_members.devops
```

3. **Debug LDAP attributes:**
```hcl
output "ldap_debug" {
  value = data.graylog_ldap_group_members.devops.members
}
```

### Permissions not applying

1. **Verify role exists:**
```bash
curl -u admin:password https://graylog.example.com/api/roles
```

2. **Check stream ID:**
```bash
terraform state show graylog_stream.app_logs
```

3. **Inspect permission resource:**
```bash
terraform state show graylog_stream_permission.devops_app_logs
```

## Migration from Manual User Management

If you have existing Graylog users:

1. **Import existing users:**
```bash
terraform import graylog_user.devops_users[\"alice\"] alice
terraform import graylog_user.devops_users[\"bob\"] bob
```

2. **Or let Terraform adopt them:**
```hcl
# Terraform will update existing users with LDAP-sourced attributes
resource "graylog_user" "ldap_synced" {
  for_each = { for m in data.graylog_ldap_group_members.devops.members : m.username => m }

  # ... configuration
}
```

## Automation: Scheduled Sync

### Option A: Cron + Terraform Cloud
```bash
# crontab entry (run daily at 2 AM)
0 2 * * * cd /path/to/terraform && terraform apply -auto-approve
```

### Option B: GitHub Actions
```yaml
name: LDAP Sync
on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM UTC
  workflow_dispatch:      # Manual trigger

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: hashicorp/setup-terraform@v3
      - name: Terraform Apply
        env:
          TF_VAR_ldap_bind_password: ${{ secrets.LDAP_PASSWORD }}
          TF_VAR_graylog_admin_password: ${{ secrets.GRAYLOG_PASSWORD }}
        run: |
          terraform init
          terraform apply -auto-approve
```

## Advanced: Nested Groups

If your LDAP has nested groups (groups containing other groups), you'll need to:

1. Query parent and child groups separately
2. Merge member lists in Terraform locals
3. Or implement recursive LDAP queries (future provider enhancement)

Example workaround:
```hcl
data "graylog_ldap_group_members" "parent_group" {
  # ... config
  group_name = "all-engineers"
}

data "graylog_ldap_group_members" "child_group" {
  # ... config
  group_name = "devops"  # nested under all-engineers
}

locals {
  all_members = distinct(concat(
    data.graylog_ldap_group_members.parent_group.members,
    data.graylog_ldap_group_members.child_group.members
  ))
}
```

## Summary

This workflow enables **Graylog OSS LDAP integration** without enterprise features:

- ✅ Automated user provisioning from LDAP
- ✅ Role-based access control (RBAC)
- ✅ Per-stream permissions
- ✅ Infrastructure as Code (auditability, version control)
- ✅ No manual user management overhead

**Next steps:**
- Set up scheduled sync (cron/CI)
- Add more LDAP groups
- Implement dashboard permissions similarly
- Monitor sync errors and user lifecycle
