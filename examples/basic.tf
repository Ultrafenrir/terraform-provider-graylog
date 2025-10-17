provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_index_set" "main" {
  title              = "main-index"
  description        = "Managed by Terraform"
  rotation_strategy  = "time"
  retention_strategy = "delete"
  shards             = 4
}

resource "graylog_input" "kafka_raw" {
  title        = "kafka-raw"
  type         = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  bind_address = "0.0.0.0"
  port         = 5555

  kafka_brokers = ["localhost:9092"]
  topic         = "logs"

  index_set_id = graylog_index_set.main.id
}

resource "graylog_stream" "s" {
  title        = "terraform-stream"
  description  = "demo"
  index_set_id = graylog_index_set.main.id

  rule {
    field = "source"
    type  = "match-exact"
    value = "terraform"
  }
}
