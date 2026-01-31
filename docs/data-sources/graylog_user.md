---
page_title: "graylog_user Data Source - Graylog"
description: |-
  Lookup a Graylog user by username.
---

# graylog_user (Data Source)

Fetches a Graylog user by `username`.

## Example Usage

```hcl
data "graylog_user" "u" {
  username = "alice"
}

output "user_email" {
  value = data.graylog_user.u.email
}
```

## Argument Reference

- `username` (String, Required) — Username of the user to lookup.

## Attributes Reference

- `full_name` — Full name
- `email` — Email
- `roles` — List of roles
- `timezone` — Timezone
- `session_timeout_ms` — Session timeout in milliseconds
- `disabled` — Whether the user is disabled
