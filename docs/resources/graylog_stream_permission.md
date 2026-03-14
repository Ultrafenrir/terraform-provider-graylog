---
page_title: "graylog_stream_permission Resource"
subcategory: "Streams"
description: |-
  Manages role-based permissions (read/edit/share) for a specific Graylog Stream.
---

# graylog_stream_permission (Resource)

Manages access permissions for a Graylog Stream via role assignment. This resource adds or removes stream-specific permissions from a role without affecting other permissions the role may have.

Use this resource to implement role-based access control (RBAC) for streams, allowing you to grant different teams or users specific access levels to individual streams.

## Example Usage

Basic read-only access:

```hcl
resource "graylog_stream_permission" "devops_read" {
  role_name = "DevOps"
  stream_id = graylog_stream.app_logs.id
  actions   = ["read"]
}
```

Read and edit access:

```hcl
resource "graylog_stream_permission" "security_readwrite" {
  role_name = "SecurityTeam"
  stream_id = graylog_stream.security_events.id
  actions   = ["read", "edit"]
}
```

Full access (read, edit, and share):

```hcl
resource "graylog_stream_permission" "admin_full" {
  role_name = "StreamAdmins"
  stream_id = graylog_stream.critical_logs.id
  actions   = ["read", "edit", "share"]
}
```

Multiple streams for the same role:

```hcl
resource "graylog_stream_permission" "devops_app_logs" {
  role_name = "DevOps"
  stream_id = graylog_stream.app_logs.id
  actions   = ["read", "edit"]
}

resource "graylog_stream_permission" "devops_app_metrics" {
  role_name = "DevOps"
  stream_id = graylog_stream.app_metrics.id
  actions   = ["read"]
}
```

Combined with LDAP user sync:

```hcl
# Read LDAP group members
data "graylog_ldap_group_members" "devops" {
  url           = "ldap://ldap.example.com:389"
  bind_dn       = "cn=readonly,dc=example,dc=com"
  bind_password = var.ldap_password
  base_dn       = "dc=example,dc=com"
  group_name    = "devops"
}

# Create Graylog users from LDAP
resource "graylog_user" "devops_users" {
  for_each = { for m in data.graylog_ldap_group_members.devops.members : m.username => m }

  username     = each.key
  email        = each.value.email
  set_password = false
  roles        = ["DevOpsRole"]
}

# Grant stream access to DevOpsRole
resource "graylog_stream_permission" "devops_streams" {
  role_name = "DevOpsRole"
  stream_id = graylog_stream.app_logs.id
  actions   = ["read", "edit"]
}
```

## Argument Reference

- `role_name` (Required, String) — Name of the Graylog role to grant permissions to. The role must already exist in Graylog.
- `stream_id` (Required, String) — ID of the stream (UUID or 24-character hex ObjectID). The stream must already exist.
- `actions` (Optional, List(String)) — List of actions to grant. Valid values: `read`, `edit`, `share`. Defaults to `["read"]` if not specified.

## Attribute Reference

- `id` (String) — Synthetic identifier in the format `role_name/stream_id`.

## Action Types

| Action | Description |
|--------|-------------|
| `read` | View stream content, search messages, view stream configuration |
| `edit` | Modify stream configuration, add/remove rules, pause/resume stream |
| `share` | Grant other users/roles access to the stream |

**Note:** Actions are cumulative. To grant read and edit, specify `["read", "edit"]`.

## Import

Import using composite identifier `role_name/stream_id` (separator can be `/` or `:`):

```bash
# Using slash separator
terraform import graylog_stream_permission.example Reader/5f1e7d1c2a3b4c5d6e7f8a9b

# Using colon separator
terraform import graylog_stream_permission.example Reader:5f1e7d1c2a3b4c5d6e7f8a9b
```

After import, run `terraform plan` to detect any drift between the imported state and your configuration.

## Notes

- This resource manages **only** the permissions for the specified `stream_id` on the specified role. Other permissions on the role remain unchanged.
- Permissions are stored in Graylog as: `streams:<action>:<stream_id>` (e.g., `streams:read:507f1f77bcf86cd799439011`).
- Deleting this resource removes the stream permissions from the role but does not delete the role or stream itself.
- If the role or stream is deleted outside Terraform, the next `terraform apply` will fail with a 404 error. Use `terraform refresh` to detect such changes.

## Permissions Matrix Example

Example setup for multi-team access control:

| Team | Role | Stream | Actions |
|------|------|--------|---------|
| DevOps | DevOpsRole | app_logs | read, edit |
| DevOps | DevOpsRole | app_metrics | read |
| Security | SecurityRole | security_events | read, edit, share |
| Security | SecurityRole | audit_logs | read, edit |
| Engineering | EngineeringRole | debug_logs | read, edit |
| Engineering | EngineeringRole | app_logs | read |

Implement with Terraform:

```hcl
locals {
  stream_permissions = {
    "DevOpsRole/app_logs"              = ["read", "edit"]
    "DevOpsRole/app_metrics"           = ["read"]
    "SecurityRole/security_events"     = ["read", "edit", "share"]
    "SecurityRole/audit_logs"          = ["read", "edit"]
    "EngineeringRole/debug_logs"       = ["read", "edit"]
    "EngineeringRole/app_logs"         = ["read"]
  }
}

resource "graylog_stream_permission" "team_access" {
  for_each = local.stream_permissions

  role_name = split("/", each.key)[0]
  stream_id = graylog_stream.streams[split("/", each.key)[1]].id
  actions   = each.value
}
```
