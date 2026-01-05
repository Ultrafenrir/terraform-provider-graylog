############################################################
# Example: Alert (Event Definition) — Threshold-based
############################################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_alert" "threshold" {
  title       = "High error rate threshold"
  description = "Triggers when ERROR count exceeds threshold"
  priority    = 2
  alert       = true

  # Pass-through config; adjust to your Graylog version/schema
  config = {
    type  = "threshold-v1"
    query = "level:ERROR"
    threshold = {
      type  = "more"
      value = 100
    }
    search_within_ms = 5 * 60 * 1000
    execute_every_ms = 1 * 60 * 1000
    group_by = ["source"]
    series = [{ id = "count", function = "count()" }]
    execution = {
      interval = { type = "interval", value = 1, unit = "MINUTES" }
    }
  }

  notification_ids = []
}
