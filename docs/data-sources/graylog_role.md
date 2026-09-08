---
page_title: "graylog_role Data Source - Graylog"
subcategory: "Users & Security"
description: |-
  Looks up a Graylog role by name, including built-in roles such as Reader.
---

# graylog_role (Data Source)

Looks up a Graylog role by name, including built-in roles such as `Reader`.

Use it wherever an API wants a role **id** rather than a role name. The important case is [graylog_auth_backend](../resources/graylog_auth_backend)'s `default_roles`.

## Example Usage

```hcl
data "graylog_role" "reader" {
  name = "Reader"
}

resource "graylog_role" "viewers" {
  name        = "viewers"
  permissions = ["streams:read", "dashboards:read"]
}

resource "graylog_auth_backend" "ldap" {
  title = "Corporate LDAP"

  default_roles = [
    data.graylog_role.reader.id,   # built-in role
    graylog_role.viewers.role_id,  # role managed here
  ]

  # ...
}
```

## Why ids and not names

`default_roles` is not validated server-side: a backend whose roles are given as names is accepted with `200` and stored verbatim. It then fails **every** login with `Authentication service unavailable` — the failure surfaces at the login attempt, far from the configuration that caused it. Passing ids avoids it.

The `/authz/roles` endpoint that exposes ids searches by substring, so `Reader` also matches `API Browser Reader`. This data source filters on an exact name.

A role you manage with the `graylog_role` resource does not need this data source: that resource exposes the same identifier as `role_id`. Its `id` remains the role name for backward compatibility.

## Argument Reference

- `name` (String, Required) — Exact role name.

## Attributes Reference

- `id` (String) — Role id, the value the authentication APIs expect.
- `description` (String) — Role description.
- `permissions` (List of String) — Permissions granted by the role.
- `read_only` (Bool) — Whether this is a built-in role.
