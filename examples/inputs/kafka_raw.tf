###############################
# Kafka Raw Input — full configuration example
#
# IMPORTANT: the field names below were verified against a live Graylog 6.0.14
# instance via:
#   GET /api/system/inputs/types/org.graylog2.inputs.raw.kafka.RawKafkaInput
# Graylog's native Kafka input does NOT use "bootstrap_servers"/"topics"/"ssl_truststore_location"/
# "sasl_mechanism"/etc. as individual top-level keys (an earlier version of this example did,
# and none of those keys ever took effect — they were silently ignored by the backend). All
# broker connection details use the field names below, and any Kafka client property beyond
# them (SSL, SASL, custom timeouts, ...) goes into the single "custom_properties" field as
# newline-separated "key=value" pairs using Kafka's own dotted property names.
#
# The other three Kafka-backed inputs (CEF Kafka, Syslog Kafka, GELF Kafka — types
# org.graylog.plugins.cef.input.CEFKafkaInput, org.graylog2.inputs.syslog.kafka.SyslogKafkaInput,
# org.graylog2.inputs.gelf.kafka.GELFKafkaInput) share this exact same connection/custom_properties
# shape; they just add codec-specific fields (e.g. GELF's decompress_size_limit).
###############################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

# --- Minimal configuration ---
resource "graylog_input" "kafka_raw_minimal" {
  title  = "kafka-raw-minimal"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = jsonencode({
    # legacy_mode's default differs by Graylog version (true on 5.x/6.x, false on 7.x, verified
    # live) — always set it explicitly to false to use the modern Kafka client and
    # bootstrap_server/custom_properties below. Forgetting this is the single most common way
    # for SSL/SASL settings to be silently ignored.
    legacy_mode      = false
    bootstrap_server = "localhost:9092" # single string: "host1:port1,host2:port2", not a list
    topic_filter     = "^logs-.*$"      # regex the topic name must match, not a literal topic
    fetch_min_bytes  = 1
    fetch_wait_max   = 100
    threads          = 2
    group_id         = "graylog-kafka-raw"
    offset_reset     = "largest" # "largest" (latest) or "smallest" (earliest)
  })
}

# --- Secure configuration (SASL_SSL with keystore/truststore certs) ---
resource "graylog_input" "kafka_raw_secure" {
  title  = "kafka-raw-secure"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = jsonencode({
    legacy_mode      = false
    bootstrap_server = "kafka1.internal:9093,kafka2.internal:9093"
    topic_filter     = "^logs-.*$"
    fetch_min_bytes  = 1
    fetch_wait_max   = 500
    threads          = 2
    group_id         = "graylog-raw-secure"
    offset_reset     = "earliest"

    # Any Kafka client property not covered by a dedicated field above (SSL, SASL, custom
    # timeouts, etc.) goes here as newline-separated "key=value" pairs using Kafka's own
    # dotted property names. Graylog itself flags this field as sensitive (it commonly holds
    # keystore/truststore passwords and JAAS config with an embedded password) — this
    # provider's `configuration` attribute is marked Sensitive for exactly that reason, so none
    # of this shows up in `terraform plan`/`apply` output.
    custom_properties = join("\n", [
      "security.protocol=SASL_SSL",
      "ssl.truststore.location=/etc/graylog/server/certs/kafka.truststore.jks",
      "ssl.truststore.password=${var.kafka_truststore_password}",
      "ssl.keystore.location=/etc/graylog/server/certs/kafka.keystore.jks",
      "ssl.keystore.password=${var.kafka_keystore_password}",
      "ssl.key.password=${var.kafka_key_password}",
      "sasl.mechanism=PLAIN",
      "sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required username=\"${var.kafka_sasl_username}\" password=\"${var.kafka_sasl_password}\";",
    ])
  })
}

variable "kafka_truststore_password" {
  type      = string
  sensitive = true
}

variable "kafka_keystore_password" {
  type      = string
  sensitive = true
}

variable "kafka_key_password" {
  type      = string
  sensitive = true
}

variable "kafka_sasl_username" {
  type      = string
  sensitive = true
}

variable "kafka_sasl_password" {
  type      = string
  sensitive = true
}
