---
page_title: "graylog_dashboard Resource - Graylog Terraform Provider"
description: |-
  Terraform Graylog provider: manage classic Graylog dashboards (automation/IaC). Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_dashboard (Resource)

Manages a classic Graylog dashboard with basic fields. Part of the Graylog Terraform Provider for Graylog automation.

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
