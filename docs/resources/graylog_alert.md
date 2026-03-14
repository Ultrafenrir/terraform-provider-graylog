---
page_title: "graylog_alert Resource - Graylog Terraform Provider"
description: |-
  Terraform Graylog provider: manage Graylog alerts (Event Definitions) with flexible config and notifications (automation/IaC). Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_alert (Resource)

Manages a Graylog Event Definition (alert). Part of the Graylog Terraform Provider for Graylog automation.

Two configuration styles are supported:
- Typed blocks (recommended) for common scenarios — `threshold` (`type = threshold-v1`) and `aggregation` (`type = aggregation-v1`). Only one typed block may be set at a time.
- Free-form `config` map (escape‑hatch) — pass any event definition payload as JSON.

## Example Usage — typed threshold (recommended)

```hcl
resource "graylog_alert" "error_rate" {
  title       = "Error rate"
  description = "Alert on error messages"
  priority    = 2
  alert       = true

  threshold {
    query            = "level:ERROR"
    search_within_ms = 5 * 60 * 1000
    execute_every_ms = 1 * 60 * 1000
    grace_ms         = 30 * 1000
    backlog_size     = 10
    group_by         = ["source"]

    # Shorthand filter: scope to specific streams
    filter {
      streams = ["<stream-id-1>", "<stream-id-2>"]
    }

    series {
      id       = "count"
      function = "count()"
    }

    threshold {
      type  = "more"
      value = 100
    }

    execution {
      interval {
        type  = "interval"
        value = 1
        unit  = "MINUTES"
      }
    }
  }

  notification_ids = []
}
```

## Example Usage — typed aggregation

```hcl
resource "graylog_alert" "agg" {
  title       = "Errors aggregation (typed)"
  description = "Aggregation example via typed block"
  alert       = true

  aggregation {
    query            = "level:ERROR"
    search_within_ms = 60 * 1000
    execute_every_ms = 60 * 1000
    grace_ms         = 15 * 1000
    backlog_size     = 3
    group_by         = ["source"]

    series {
      id       = "count"
      function = "count()"
    }

    # Optional threshold condition for aggregation
    threshold {
      type  = "more"
      value = 0
    }

    execution {
      interval {
        type  = "interval"
        value = 1
        unit  = "MINUTES"
      }
    }
  }
}
```

## Example Usage — free‑form config (escape‑hatch)

```hcl
resource "graylog_alert" "agg" {
  title       = "Errors aggregation"
  description = "Aggregation example via raw config"
  alert       = true

  config = jsonencode({
    type   = "aggregation-v1"
    query  = "level:ERROR"
    series = [{ id = "count", function = "count()" }]
    group_by = ["source"]
    execution = {
      interval = { type = "interval", value = 1, unit = "MINUTES" }
    }
  })
}
```

## Argument Reference

- `title` (String, Required) — Event title.
- `description` (String, Optional) — Description.
- `priority` (Int, Optional) — Priority/severity.
- `alert` (Boolean, Optional) — Whether to create alerts.
- `threshold` (Block, Optional) — Typed configuration for threshold‑based alerts (`type = threshold-v1`).
  - `query` (String, Optional) — Graylog query.
  - `search_within_ms` (Int, Optional) — Search window in ms.
  - `execute_every_ms` (Int, Optional) — Execution interval in ms.
  - `grace_ms` (Int, Optional) — Grace period in milliseconds before triggering notifications.
  - `backlog_size` (Int, Optional) — Number of backlog messages to collect.
  - `group_by` (List(String), Optional) — Group-by fields.
  - `filter` (Block, Optional) — Shorthand filters.
    - `streams` (List(String), Optional) — Stream IDs to scope the search to (maps to payload `streams`).
  - `series` (Block, Optional, repeatable) — Aggregation series.
    - `id` (String, Optional) — Series ID.
    - `function` (String, Required) — Aggregation function, e.g. `count()`.
  - `threshold` (Block, Required) — Threshold condition.
    - `type` (String, Required) — Condition type, e.g. `more`/`less`.
    - `value` (Float, Required) — Threshold value.
  - `execution` (Block, Optional) — Execution schedule.
    - `interval` (Block, Optional) — Interval settings.
      - `type` (String, Optional)
      - `value` (Int, Optional)
      - `unit` (String, Optional)
- `config` (String, Optional) — Free‑form configuration as JSON string (`jsonencode({...})`) for any event definition payload.
- `notification_ids` (List(String), Optional) — Notification IDs to trigger.
- `timeouts` (Block, Optional) — Customize create/update/delete timeouts.

Additionally, a typed `aggregation` block (Optional) mirrors the structure of `threshold` and maps to `type = aggregation-v1`:

- `aggregation` (Block, Optional) — Typed configuration for aggregation‑based alerts (`type = aggregation-v1`). Only one of `threshold` or `aggregation` may be set.
  - `query` (String, Optional) — Graylog query.
  - `search_within_ms` (Int, Optional) — Search window in ms.
  - `execute_every_ms` (Int, Optional) — Execution interval in ms.
  - `grace_ms` (Int, Optional) — Grace period in milliseconds before triggering notifications.
  - `backlog_size` (Int, Optional) — Number of backlog messages to collect.
  - `group_by` (List(String), Optional) — Group-by fields.
  - `filter` (Block, Optional) — Shorthand filters.
    - `streams` (List(String), Optional) — Stream IDs to scope the search to (maps to payload `streams`).
  - `series` (Block, Optional, repeatable) — Aggregation series (`id`, `function`).
  - `threshold` (Block, Optional) — Optional threshold for aggregation (`type`, `value`).
  - `execution` (Block, Optional) — Execution schedule with `interval { type, value, unit }`.

## Attributes Reference

- `id` — Event Definition ID.

## Notes
- Если указаны и typed‑блоки, и `config`, провайдер отдаёт приоритет typed‑блокам и приводит `config` в состоянии к каноническому JSON эквивалентной конфигурации.
- Чтобы избежать неожиданного дрейфа после `import` ресурсов, провайдер не заполняет typed‑блоки из `config` автоматически, если соответствующий блок отсутствовал в состоянии/плане.

## Import

```bash
terraform import graylog_alert.a <event_definition_id>
```
