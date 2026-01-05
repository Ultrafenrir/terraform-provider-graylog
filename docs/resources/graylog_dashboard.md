---
page_title: "graylog_dashboard Resource - Graylog"
description: |-
  Manages a classic Graylog dashboard (title and description).
---

# graylog_dashboard (Resource)

Manages a classic Graylog dashboard with basic fields.

## Example Usage

```hcl
resource "graylog_dashboard" "ops" {
  title       = "Ops overview"
  description = "Classic dashboard"
}
```

## Argument Reference

- `title` (String, Required) — Dashboard title.
- `description` (String, Optional) — Dashboard description.
- `timeouts` (Block, Optional) — Customize create/update/delete timeouts.

## Attributes Reference

- `id` — Dashboard ID.

## Import

```bash
terraform import graylog_dashboard.d <dashboard_id>
```
