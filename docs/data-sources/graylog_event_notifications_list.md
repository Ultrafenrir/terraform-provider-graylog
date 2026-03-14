---
page_title: "graylog_event_notifications Data Source - Graylog"
description: |-
  Lists Graylog event notifications and provides both full item objects and a title→id map.
---

# graylog_event_notifications (Data Source)

Lists event notifications and returns:
- `items` — list of objects (`id`, `title`, `type`, `description`).
- `title_map` — convenience map `title -> id`.

## Example Usage

```hcl
data "graylog_event_notifications" "all" {}

output "notifications_map" {
  value = data.graylog_event_notifications.all.title_map
}
```

## Attributes Reference

- `items` (List(Object)) — fields:
  - `id` (String)
  - `title` (String)
  - `type` (String)
  - `description` (String)
- `title_map` (Map(String)) — `title -> id` map.
