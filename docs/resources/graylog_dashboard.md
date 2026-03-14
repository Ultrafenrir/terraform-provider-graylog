---
page_title: "graylog_dashboard Resource - Graylog Terraform Provider"
subcategory: "Dashboards"
description: |-
  Terraform Graylog provider: manage classic Graylog dashboards (automation/IaC). Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_dashboard (Resource)

Manages a classic Graylog dashboard with basic fields. Part of the Graylog Terraform Provider for Graylog automation.

> Note: This resource is gated by capability detection. On images/versions that do not expose classic dashboards CRUD, create/update will fail early with a clear error. See the capability table in `docs/index.md`.

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

You can import by ID (UUID/24-hex) or by exact title. For title-based import, use the explicit `title:` prefix. If multiple dashboards share the same title, import by ID.

```bash
# By ID
terraform import graylog_dashboard.d 5f3c2a0b9e0f1a2b3c4d5e6f

# By title (exact match)
terraform import graylog_dashboard.d "title:Ops overview"
```
