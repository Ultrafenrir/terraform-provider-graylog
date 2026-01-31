terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = ">= 0.1.0"
    }
  }
}

provider "graylog" {
  url   = var.graylog_url
  token = var.graylog_token
}

variable "graylog_url" { type = string }
variable "graylog_token" { type = string }

resource "graylog_dashboard" "main" {
  title       = "Main Dashboard"
  description = "Key service metrics"
}

resource "graylog_dashboard_widget" "total_errors" {
  dashboard_id = graylog_dashboard.main.id
  type         = "SEARCH_RESULT_COUNT"
  description  = "Total error messages"
  cache_time   = 10

  configuration = jsonencode({
    timerange = { type = "relative", range = 300 }
    query     = "level:ERROR"
  })
}
