########################################
# Alert (Event Definition) aggregation
########################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_alert" "error_rate" {
  title       = "Error rate"
  description = "Alert on error messages"
  priority    = 2
  alert       = true

  # Free-form config (pass-through). This is an example; adjust to your Graylog version.
  config = {
    type   = "aggregation-v1"
    query  = "level:ERROR"
    series = [
      { id = "count", function = "count()" }
    ]
    group_by = ["source"]
    execution = {
      interval = { type = "interval", value = 1, unit = "MINUTES" }
    }
  }

  # Provide notification IDs that already exist in Graylog
  notification_ids = []
}
