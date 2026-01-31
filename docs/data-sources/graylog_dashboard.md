---
page_title: "graylog_dashboard Data Source - Graylog"
description: |-
  Lookup a Graylog classic dashboard by ID.
---

# graylog_dashboard (Data Source)

Fetches a classic Graylog dashboard by `id`.

## Example Usage

```hcl
data "graylog_dashboard" "d" {
  id = "5f4d2e8aa1234bcdef567890"
}

output "dash_title" {
  value = data.graylog_dashboard.d.title
}
```

## Argument Reference

- `id` (String, Required) — Dashboard ID.

## Attributes Reference

- `title` — Dashboard title
- `description` — Dashboard description
