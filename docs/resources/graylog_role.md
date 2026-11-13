---
page_title: "graylog_role Resource - Graylog Terraform Provider"
subcategory: "Users & Security"
description: |-
  Manages a Graylog role and its permissions.
---

# graylog_role (Resource)

Manages a custom Graylog role, including its description and permissions.

## Example Usage

```hcl
resource "graylog_role" "readonly" {
  name        = "tf-readonly"
  description = "Readonly role managed by Terraform"
  permissions = [
    "dashboards:read",
    "indices:read",
  ]
}
```

## Argument Reference

- `name` (String, Required) — Immutable role name.
- `description` (String, Optional) — Role description.
- `permissions` (List of String, Optional) — Role permissions.

## Attributes Reference

- `id` (String) — Terraform identifier, equal to `name`.
- `role_id` (String) — Mongo id of the role. Pass this wherever an API wants a role identifier — `graylog_auth_backend.default_roles` in particular, which accepts a name without complaint and then breaks every login. `id` remains the role name for backward compatibility.
- `read_only` (Boolean) — Whether this is a system role that cannot be modified.

## Import

```shell
terraform import graylog_role.readonly tf-readonly
```
