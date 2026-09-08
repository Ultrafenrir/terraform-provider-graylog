---
page_title: "graylog_ldap_group_members Data Source - Graylog"
subcategory: "Users & Security"
description: |-
  Reads members of an LDAP group to drive Graylog user/role automation (safe, read-only). Combine with roles and stream permissions.
---

# graylog_ldap_group_members

Reads members of an LDAP group by name. This is a safe, read-only helper to build user management flows (e.g., creating Graylog users with `for_each`).

Note for Graylog OSS users (Graylog LDAP integration / sync LDAP groups to Graylog):

- Graylog OSS does not include a built‑in, policy‑driven "sync users from LDAP" feature. Use this data source to implement "Graylog LDAP integration": fetch members of an LDAP group, then create/update Graylog users and assign roles with Terraform. Combine with `graylog_stream_permission` to grant per‑stream access to those roles.
  - Keywords: graylog ldap integration, graylog sync ldap groups to graylog, graylog ldap groups sync, graylog user sync ldap.

The data source connects directly to LDAP; it does not use Graylog’s LDAP settings.

## Example Usage

```hcl
data "graylog_ldap_group_members" "devops" {
  url           = "ldap://127.0.0.1:389"  # or ldaps://host:636
  bind_dn       = "cn=admin,dc=example,dc=org"
  bind_password = "admin"
  base_dn       = "dc=example,dc=org"
  group_name    = "devops"
}

output "devops_members" {
  value = data.graylog_ldap_group_members.devops.members
}
```

Using results to create users:

```hcl
# Graylog requires a password to create a profile; it is sent once and only
# re-sent when the value changes, so later applies never push one to LDAP users
resource "random_password" "ldap_synced" {
  for_each = { for m in data.graylog_ldap_group_members.devops.members : m.username => m }
  length   = 32
}

resource "graylog_user" "ldap_synced" {
  for_each = { for m in data.graylog_ldap_group_members.devops.members : m.username => m }

  username = each.key
  email    = each.value.email
  # Graylog requires at least two words here; the directory overwrites it on login
  full_name = coalesce(each.value.display_name, "${each.key} (directory)")
  password  = random_password.ldap_synced[each.key].result

  lifecycle {
    ignore_changes = [full_name, email] # the directory overwrites these on login
  }
}
```

Sync roles from LDAP group (basic role mapping example):

```hcl
# Suppose members of LDAP group "devops" should get Graylog roles Reader + PowerUser
locals {
  devops_roles = ["Reader", "PowerUser"]
}

resource "graylog_user" "ldap_synced_with_roles" {
  for_each = { for m in data.graylog_ldap_group_members.devops.members : m.username => m }

  username = each.key
  email    = each.value.email
  # Graylog requires at least two words here; the directory overwrites it on login
  full_name = coalesce(each.value.display_name, "${each.key} (directory)")
  password  = random_password.ldap_synced[each.key].result # bootstrap password from the example above

  # Assign roles derived from LDAP group
  roles = local.devops_roles

  lifecycle {
    ignore_changes = [full_name, email] # the directory overwrites these on login
  }
}
```

Mapping roles to Stream permissions (grant role access to a Stream):

```hcl
resource "graylog_stream_permission" "devops_can_read" {
  role_name = "DevOps"          # the role you assigned to LDAP users
  stream_id = var.stream_id      # target stream ID
  actions   = ["read"]          # or ["read", "edit", "share"]
}
```

## Argument Reference

- `url` (Required) — LDAP URL, e.g. `ldap://host:389` or `ldaps://host:636`.
- `bind_dn` (Required) — Bind DN.
- `bind_password` (Required, Sensitive) — Bind password.
- `base_dn` (Required) — Search base DN.
- `group_name` (Required) — Group common name (cn) to resolve.
- `starttls` (Optional) — Use StartTLS over plain LDAP.
- `insecure` (Optional) — Skip TLS verification (alias of `insecure_skip_verify`; use for self‑signed certs in dev/test).
- `insecure_skip_verify` (Optional) — Skip TLS verification (legacy name).

Attribute mapping (optional overrides; sensible defaults for `groupOfNames`/`inetOrgPerson`):
- `group_filter` (default `(cn=%s)`) — Group search filter; `%s` is replaced with the escaped `group_name`.
- `member_attr` (default `member`) — Group attribute containing member DNs.
- `user_filter` (default `(objectClass=inetOrgPerson)`) — Applied when reading user entries.
- `user_id_attr` (default `uid`) — Emitted as `username`.
- `email_attr` (default `mail`)
- `display_name_attr` (default `cn`)

## Attributes Reference

- `id` — Synthetic ID in the form `<group_name>@<base_dn>`.
- `members` — List of objects with fields: `username`, `dn`, `email`, `display_name`.
