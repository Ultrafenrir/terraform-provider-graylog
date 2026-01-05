###############################
# Kafka Raw Input example
###############################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_input" "kafka_raw" {
  title  = "kafka-raw"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = {
    bootstrap_servers = ["localhost:9092"]
    topics            = ["logs"]
    fetch_min_bytes   = 1
  }
}
