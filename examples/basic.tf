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
  title        = "main-index"
  description  = "Managed by Terraform"
  index_prefix = "main"
  shards       = 4
  replicas     = 1

  rotation {
    class = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
    config = {
      max_docs_per_index = "20000000"
    }
  }

  retention {
    class = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = {
      max_number_of_indices = "20"
    }
  }
}

# Input example (Raw/Plaintext Kafka) — uses flexible configuration map.
# Field names verified against a live Graylog 6.0.14 instance via
# GET /api/system/inputs/types/org.graylog2.inputs.raw.kafka.RawKafkaInput — see
# examples/inputs/kafka_raw.tf for the full field reference and SSL/SASL setup.
resource "graylog_input" "kafka_raw" {
  title  = "kafka-raw"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = jsonencode({
    legacy_mode      = false # required: false = modern KafkaConsumer client, true (default) = old ZooKeeper consumer
    bootstrap_server = "localhost:9092"
    topic_filter     = "^logs-.*$" # regex, not a literal topic name
    fetch_min_bytes  = 1
    fetch_wait_max   = 100
    threads          = 2
    group_id         = "graylog"
  })

  # Optional extractor (see docs/resources/graylog_input.md for all fields)
  extractor {
    title          = "extract user"
    extractor_type = "regex"
    source_field   = "message"
    target_field   = "user"
    extractor_config = jsonencode({
      regex_value = "user=(\\w+)"
    })
  }
}

# Stream with rules (type is an integer enum in Graylog)
resource "graylog_stream" "s" {
  title        = "terraform-stream"
  description  = "demo"
  index_set_id = graylog_index_set.main.id

  rule {
    field = "source"
    type  = 1 # equals / exact match
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

  # Free-form config passed as-is to Graylog (JSON-encoded string, like graylog_input's
  # configuration attribute)
  config = jsonencode({
    type     = "aggregation-v1"
    query    = "level:ERROR"
    series   = [{ id = "count", function = "count()" }]
    group_by = ["source"]
    execution = {
      interval = { type = "interval", value = 1, unit = "MINUTES" }
    }
  })

  # Provide existing notification IDs if any
  notification_ids = []
}
