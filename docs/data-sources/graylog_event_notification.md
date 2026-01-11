---
page_title: "graylog_event_notification Data Source - Graylog"
description: |-
  Lookup a Graylog Event Notification by ID.
---

# graylog_event_notification (Data Source)

Fetches a Graylog Event Notification by `id`.

## Example Usage

```hcl
data "graylog_event_notification" "n" {
  id = "64b7c8e2e4b0a1234567890a"
}

output "notif_type" {
  value = data.graylog_event_notification.n.type
}
```

## Argument Reference

- `id` (String, Required) — ID of the event notification.

## Attributes Reference

- `title` — Title
- `type` — Type (email/http/slack/pagerduty)
- `description` — Description
- `config` — JSON-encoded configuration object
