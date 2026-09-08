---
page_title: "graylog_user Resource - Graylog Terraform Provider"
subcategory: "Users & Security"
description: |-
  Terraform Graylog provider: manage Graylog users (roles/password), including pre-created LDAP/AD profiles. Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_user (Resource)

Manages a Graylog user: a local account, or the pre-created profile of an external (LDAP/Active Directory) user whose roles you want Terraform to own. Part of the Graylog Terraform Provider for Graylog automation. Supports Graylog v5, v6, and v7.

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

  # Required by Graylog on create; only re-sent when the value changes
  password = var.alice_password
}
```

### Pre-creating a directory (LDAP/AD) user

Graylog binds the first directory login to an existing profile with the same username: the profile keeps its id and roles and is flagged `external`. This lets Terraform own roles such as `Admin` instead of granting them after the first login.

```hcl
resource "graylog_user" "alice" {
  username  = "alice"
  full_name = "Alice Doe"         # two words; the directory overwrites it on login
  email     = "alice@example.org" # the directory overwrites it on login
  roles     = ["Admin", "Reader"] # Terraform-owned, kept across logins

  # Graylog refuses to create a user without a password. It is sent once on
  # create; later applies (e.g. a roles change) never touch it, so the 403
  # "Cannot change password for external user" does not occur.
  password = var.bootstrap_password

  lifecycle {
    ignore_changes = [full_name, email] # or set them to the directory values
  }
}
```

Notes for external users:

- The directory owns `full_name` and `email`: Graylog overwrites both from the directory attributes on every login. Either set them to the directory values or exclude them with `lifecycle { ignore_changes = [...] }`.
- Roles are Terraform-owned. Graylog does not merge an authentication backend's `default_roles` into a pre-existing profile, so list every role the user needs here.
- An existing profile created by a previous login can be adopted with `terraform import` and then managed the same way (leave `password` unset).

## Argument Reference

- `username` (String, Required) — Username (immutable).
- `full_name` (String, Optional) — Full name. Graylog requires at least two words (first and last name) on create, otherwise it answers `A lastName value is required`. See the external-user notes above.
- `email` (String, Optional) — Email. See the external-user notes above.
- `roles` (List(String), Optional) — Roles assigned to the user.
- `timezone` (String, Optional) — Timezone (e.g., `UTC`).
- `session_timeout_ms` (Number, Optional, Computed) — Session timeout in milliseconds. When unset Graylog applies its default (8 hours) and the value is read back into state. `0` is rejected: Graylog accepts it, but every interactive login then fails with `Session timeout is set to 0 seconds`. Users created by provider versions before this rule have `0` stored server-side; set the attribute explicitly once to repair them.
- `disabled` (Boolean, Optional, Computed) — Disable the user account. When unset the server's current state is read back into state and left as it is, so omitting it is not a change.
- `password` (String, Optional, Computed, Sensitive) — Password. Required by Graylog on create. Graylog never returns it, so the provider keeps the last applied value in state and calls the password endpoint only when the configured value differs from it. Removing the attribute from the configuration keeps the state value and is not a change; re-adding the same value is a no-op.

## Attributes Reference

- `id` — Same as `username`.

## Import

Import by username:

```bash
terraform import graylog_user.alice alice
```

An imported user has no `password` in state; leave the attribute unset (external users) or set it to make the provider apply one on the next apply.
