# Example: Event Definition linked with Notification

terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = "~> 0.1"
    }
  }
}

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "${base64encode("admin:admin")}" # or API token
}

resource "graylog_event_notification" "email_notify" {
  title = "tf-example-email-notify"
  type  = "email"
  config = jsonencode({
    sender           = "noreply@example.com"
    subject          = "Alert: ${title}"
    body_template    = "Triggered: ${title}"
    user_recipients  = ["admin"]
    email_recipients = ["root@example.com"]
  })
}

resource "graylog_alert" "failed_logins" {
  title       = "Failed logins"
  description = "Failed logins over threshold"
  priority    = 2
  alert       = true

  # Minimal config example (adapt to your env)
  config = jsonencode({
    type  = "aggregation-v1"
    query = "event_type:failed_login"
    group_by = []
    series = [
      { id = "count", function = "count()" }
    ]
    conditions = {
      expression = {
        left  = { expr = { type = "number-ref", ref = "count" } }
        right = { expr = { type = "number", value = 10 } }
        expr  = { type = "greater" }
      }
    }
    search_within_ms = 300000
    execute_every_ms = 60000
  })

  notification_ids = [graylog_event_notification.email_notify.id]
}
