---
page_title: "graylog_alert Resource - Graylog"
description: |-
  Manages a Graylog Event Definition (alerts) with flexible `config` map and optional `notification_ids`.
---

# graylog_alert (Resource)

Manages a Graylog Event Definition (alert). The `config` attribute is a free-form map passed as-is to Graylog, supporting various event types (e.g., aggregation, threshold).

## Example Usage

```hcl
resource "graylog_alert" "error_rate" {
  title       = "Error rate"
  description = "Alert on error messages"
  priority    = 2
  alert       = true

  config = {
    type   = "aggregation-v1"
    query  = "level:ERROR"
    series = [{ id = "count", function = "count()" }]
    group_by = ["source"]
    execution = {
      interval = { type = "interval", value = 1, unit = "MINUTES" }
    }
  }

  notification_ids = []
}
```

## Argument Reference

- `title` (String, Required) — Event title.
- `description` (String, Optional) — Description.
- `priority` (Int, Optional) — Priority/severity.
- `alert` (Boolean, Optional) — Whether to create alerts.
- `config` (Map(dynamic), Optional) — Free-form configuration map; values may be strings, numbers, booleans or nested objects.
- `notification_ids` (List(String), Optional) — Notification IDs to trigger.
- `timeouts` (Block, Optional) — Customize create/update/delete timeouts.

## Attributes Reference

- `id` — Event Definition ID.

## Import

```bash
terraform import graylog_alert.a <event_definition_id>
```
