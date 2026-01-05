############################################################
# Basic examples for all main resources of this provider
# Compatible with Graylog v5/v6/v7 (see docker-compose.yml)
############################################################

provider "graylog" {
  # For docker-compose default stack use:
  # url   = "http://localhost:9000/api"
  # token = base64("admin:admin") or API token
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

# Index set used by stream
resource "graylog_index_set" "main" {
  title              = "main-index"
  description        = "Managed by Terraform"
  rotation_strategy  = "time"
  retention_strategy = "delete"
  shards             = 4
}

# Input example (Kafka JSON) — uses flexible configuration map
resource "graylog_input" "kafka_json" {
  title = "kafka-json"
  type  = "org.graylog.plugins.kafka.input.KafkaJsonInput"
  global = true

  configuration = {
    bootstrap_servers        = ["localhost:9092"]
    topic_filter             = "logs-*"
    fetch_min_bytes          = 1
    allow_auto_create_topics = false
  }

  # Optional extractors (free-form maps passed as-is to the API)
  extractors = [
    {
      type         = "regex"
      title        = "extract user"
      target_field = "user"
      source_field = "message"
      regex_value  = "user=(\\w+)"
    }
  ]
}

# Stream with rules (type is an integer enum in Graylog)
resource "graylog_stream" "s" {
  title        = "terraform-stream"
  description  = "demo"
  index_set_id = graylog_index_set.main.id

  rule {
    field = "source"
    type  = 1              # equals / exact match
    value = "terraform"
  }
}

# Pipeline (classic pipelines)
resource "graylog_pipeline" "p1" {
  title       = "sanitize"
  description = "Sample pipeline"
  # Full pipeline definition as Graylog source string
  source = <<-EOT
    pipeline "sanitize"
    stage 0 match either
    rule "drop_empty";

    rule "drop_empty"
    when
      to_string($message.message) == ""
    then
      drop_message();
    end
  EOT
}

# Dashboard (classic)
resource "graylog_dashboard" "d1" {
  title       = "Ops overview"
  description = "Classic dashboard"
}

# Alert (Event Definition)
resource "graylog_alert" "a1" {
  title       = "Error rate"
  description = "Alert on error messages"
  priority    = 2
  alert       = true

  # Free-form config passed as-is to Graylog
  config = {
    type   = "aggregation-v1"
    query  = "level:ERROR"
    series = [{ id = "count", function = "count()" }]
    group_by = ["source"]
    execution = {
      interval = { type = "interval", value = 1, unit = "MINUTES" }
    }
  }

  # Provide existing notification IDs if any
  notification_ids = []
}
