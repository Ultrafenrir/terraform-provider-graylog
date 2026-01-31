---
page_title: "graylog_event_notification Resource"
subcategory: "Events"
description: |-
  Manages a Graylog Event Notification (email, http, slack, pagerduty).
---

# graylog_event_notification (Resource)

Manages a Graylog Event Notification. Supports Graylog v5, v6, and v7.

## Example Usage

```hcl
resource "graylog_event_notification" "email_ops" {
  title = "Ops Email"
  type  = "email"
  # Config depends on type; pass as JSON string
  config = jsonencode({
    sender           = "graylog@example.com"
    subject          = "Graylog Alert"
    body_template    = "Alert: ${event_definition_title}"
    user_recipients  = []
    email_recipients = ["ops@example.com"]
  })
}

resource "graylog_event_notification" "http_hook" {
  title = "Webhook"
  type  = "http"
  config = jsonencode({
    url     = "https://hooks.example.com/graylog"
    method  = "POST"
    content_type = "application/json"
    headers = { "X-Token" = "s3cret" }
    body_template = jsonencode({
      event = "${event_definition_title}"
      message = "${event_definition_description}"
    })
  })
}

resource "graylog_event_notification" "slack_alerts" {
  title = "Slack Alerts"
  type  = "slack"
  config = jsonencode({
    webhook_url = "https://hooks.slack.com/services/XXX/YYY/ZZZ"
    channel     = "#alerts"
    custom_message = "${event_definition_title}: ${backlog}"
  })
}

resource "graylog_event_notification" "pagerduty_p1" {
  title = "PagerDuty P1"
  type  = "pagerduty"
  config = jsonencode({
    routing_key = "pd-routing-key"
    severity    = "critical"
    custom_incident_key_template = "${event_definition_id}:${id}"
  })
}
```

## Argument Reference

- `title` (String, Required) — Notification title.
- `type` (String, Required) — One of `email`, `http`, `slack`, `pagerduty`.
- `description` (String, Optional) — Description.
- `config` (String, Required) — JSON-encoded config for the selected type.

## Import

Import by ID:

```bash
terraform import graylog_event_notification.this <notification_id>
```
