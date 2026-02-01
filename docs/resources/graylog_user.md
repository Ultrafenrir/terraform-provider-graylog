---
page_title: "graylog_user Resource - Graylog Terraform Provider"
subcategory: "Security"
description: |-
  Terraform Graylog provider: manage Graylog local users (roles/password). Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_user (Resource)

Manages a Graylog local user. Part of the Graylog Terraform Provider for Graylog automation. Supports Graylog v5, v6, and v7.

## Example Usage

```hcl
resource "graylog_user" "alice" {
  username = "alice"
  full_name = "Alice Doe"
  email = "alice@example.com"
  roles = ["Reader", "PowerUser"]
  timezone = "UTC"
  session_timeout_ms = 3600000
  disabled = false

  # Password is sensitive and write-only; not read back from API
  password = var.alice_password
}
```

## Argument Reference

- `username` (String, Required) — Username (immutable).
- `full_name` (String, Optional) — Full name.
- `email` (String, Optional) — Email.
- `roles` (List(String), Optional) — Roles assigned to the user.
- `timezone` (String, Optional) — Timezone (e.g., `UTC`).
- `session_timeout_ms` (Number, Optional) — Session timeout in milliseconds.
- `disabled` (Boolean, Optional) — Disable the user account.
- `password` (String, Optional, Sensitive) — Write-only password. If set/changed, provider will call password update endpoint.

## Attributes Reference

- `id` — Same as `username`.

## Import

Import by username:

```bash
terraform import graylog_user.alice alice
```
