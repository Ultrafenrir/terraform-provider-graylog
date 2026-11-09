terraform {
  required_providers {
    graylog = {
      source = "Ultrafenrir/graylog"
      # Requires a release containing index-set Data Tiering support.
    }
  }
}

variable "graylog_url" {
  type = string
}

variable "graylog_api_token" {
  type      = string
  sensitive = true
}

provider "graylog" {
  url         = var.graylog_url
  auth_method = "basic_token"
  api_token   = var.graylog_api_token
}

resource "graylog_index_set" "application" {
  title        = "Application logs"
  index_prefix = "application_logs"
  shards       = 1
  replicas     = 0

  use_legacy_rotation = false
  data_tiering = jsonencode({
    type               = "hot_only"
    index_lifetime_min = "P30D"
    index_lifetime_max = "P40D"
  })
}
