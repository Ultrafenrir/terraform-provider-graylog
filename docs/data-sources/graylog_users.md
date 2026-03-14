---
page_title: "graylog_users Data Source - Graylog"
description: |-
  Lists Graylog users and provides both full item objects and a username→id map for convenient lookups.
---

# graylog_users (Data Source)

Lists Graylog users and returns:
- `items` — list of user objects (`id`, `username`, `full_name`, `email`, `disabled`).
- `title_map` — convenience map `username -> id` (on older versions without stable IDs, maps to `username`).

## Example Usage

```hcl
data "graylog_users" "all" {}

output "users_map" {
  value = data.graylog_users.all.title_map
}

output "first_user" {
  value = try(data.graylog_users.all.items[0], null)
}
```

## Attributes Reference

- `items` (List(Object)) — list of objects with fields:
  - `id` (String)
  - `username` (String)
  - `full_name` (String)
  - `email` (String)
  - `disabled` (Bool)
- `title_map` (Map(String)) — `username -> id` map (or `username` if `id` is not available).
