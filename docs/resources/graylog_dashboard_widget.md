---
page_title: "graylog_dashboard_widget Resource - Graylog Terraform Provider"
subcategory: "Dashboards"
description: |-
  Terraform Graylog provider: manage classic Graylog dashboard widgets (automation/IaC). Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_dashboard_widget (Resource)

> **Deprecated.** This resource uses the legacy classic-dashboard widget API,
> which is unavailable in supported Graylog OSS releases. Existing state
> remains readable for compatibility. Do not use this resource for new
> configurations; modern dashboard widgets are stored in Views.

Manages a widget through Graylog's legacy classic-dashboard widget API. Current
Graylog 5.2, 6.x, and 7.x OSS images used by this repository do not expose that
API, so this resource cannot be used with those images. Modern dashboard
widgets are stored in Views state and need a separate Views-based resource.

Note: This resource targets classic dashboards (not Views). If you need Views widgets, consider opening an issue for `views` support.

> Capability note: Creation/updates require classic dashboards CRUD support. On images/versions without legacy dashboards, operations will fail fast with a clear error. See the capability table in `docs/index.md`.

## Example Usage

```hcl
resource "graylog_dashboard" "main" {
  title       = "Main Dashboard"
  description = "Key service metrics"
}

resource "graylog_dashboard_widget" "total_errors" {
  dashboard_id = graylog_dashboard.main.id
  type         = "SEARCH_RESULT_COUNT"
  description  = "Total error messages"
  cache_time   = 10

  # Widget configuration depends on widget type; pass as JSON string
  configuration = jsonencode({
    timerange = { type = "relative", range = 300 }
    query     = "level:ERROR"
  })
}
```

## Argument Reference

- `dashboard_id` (String, Required) — The ID of the dashboard to attach the widget to.
- `type` (String, Required) — Widget type (e.g., `SEARCH_RESULT_COUNT`).
- `description` (String, Optional) — Widget description.
- `cache_time` (Number, Optional) — Cache time in seconds.
- `configuration` (String, Required) — JSON-encoded configuration object specific to the widget type.

## Attributes Reference

- `id` — Widget ID.

## Import

Import by widget ID (the dashboard_id must be provided in config after import):

```bash
terraform import graylog_dashboard_widget.total_errors <widget_id>
```
