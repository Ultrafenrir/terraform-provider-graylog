---
page_title: "graylog_input Resource - Graylog"
description: |-
  Manages a Graylog input. Supports flexible configuration map for all input types (incl. Kafka) and optional extractors.
---

# graylog_input (Resource)

Manages a Graylog input. Compatible with Graylog v5/v6/v7. The `configuration` attribute is a free-form map supporting strings, numbers, booleans, lists and nested objects, covering all input types (including Kafka inputs). Extractors can be managed alongside the input.

## Example Usage

```hcl
resource "graylog_input" "kafka_json" {
  title  = "kafka-json"
  type   = "org.graylog.plugins.kafka.input.KafkaJsonInput"
  global = true

  configuration = {
    bootstrap_servers        = ["localhost:9092"]
    topic_filter             = "logs-*"
    allow_auto_create_topics = false
  }

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
```

## Argument Reference

- `title` (String, Required) — Input title.
- `type` (String, Required) — Fully qualified input class (e.g. `org.graylog2.inputs.syslog.udp.SyslogUDPInput`).
- `global` (Boolean, Optional) — Whether the input is global.
- `node` (String, Optional) — Node ID to run the input on when not global.
- `configuration` (Map(dynamic), Optional) — Free-form configuration map. Values may be strings, numbers, booleans, lists, or nested objects.
- `extractors` (List(Map(any)), Optional) — List of extractor objects passed to Graylog as-is. Use either top-level fields or a nested `data` map.
- `timeouts` (Block, Optional) — Customize create/update/delete timeouts.

## Attributes Reference

- `id` — Input ID.

## Import

```bash
terraform import graylog_input.i <input_id>
```

---

## Kafka Raw input configuration

Kafka Raw Input class: `org.graylog2.inputs.raw.kafka.RawKafkaInput`.

The `configuration` map accepts common Kafka consumer options (depending on Graylog/plugin version). Below is a consolidated list of commonly supported keys with brief descriptions. Types are indicative; pass values in appropriate Terraform types (string/int/bool/list(string)). Some keys may not be available in all Graylog versions.

### Connection / topics
- `bootstrap_servers` (list(string), required) — Kafka brokers, e.g. `["kafka:9092"]`.
- `topics` (list(string), optional) — Explicit list of topics to subscribe to.
- `topic_pattern` (string, optional) — Regex pattern of topics to subscribe to (mutually exclusive with `topics`).
- `topic_filter` (string, optional) — Wildcard filter supported in some Graylog versions; alternative to `topic_pattern`.

### Consumer core
- `group_id` (string) — Consumer group id.
- `client_id` (string) — Optional client identifier.
- `auto_offset_reset` (string) — Behavior when no offset is present: `earliest` | `latest` | `none`.
- `enable_auto_commit` (bool) — Enable periodic commits of offsets (if exposed by version).
- `auto_commit_interval_ms` (int) — Interval for auto commits.
- `allow_auto_create_topics` (bool) — Allow broker to auto-create topics.

### Poll/fetch and throughput
- `max_poll_records` (int) — Max records returned in a single poll.
- `max_poll_interval_ms` (int) — Max delay between polls before rebalancing.
- `poll_timeout_ms` (int) — Poll timeout.
- `fetch_min_bytes` (int) — Minimum fetch size in bytes.
- `fetch_max_bytes` (int) — Max bytes fetched per request.
- `max_partition_fetch_bytes` (int) — Max bytes fetched per partition per request.
- `fetch_max_wait_ms` (int) — Max wait time for fetch when data is insufficient.

### Session/heartbeat
- `session_timeout_ms` (int) — Consumer group session timeout.
- `heartbeat_interval_ms` (int) — Heartbeat interval.

### Retries/backoff
- `retries` (int) — Number of retries on transient errors.
- `retry_backoff_ms` (int) — Backoff between retries.
- `reconnect_backoff_ms` (int) — Backoff for reconnects.

### Network/buffers/timeouts
- `connections_max_idle_ms` (int) — Close idle connections after this time.
- `receive_buffer_bytes` (int) — TCP receive buffer size.
- `send_buffer_bytes` (int) — TCP send buffer size.
- `request_timeout_ms` (int) — Request timeout for the client.

### Processing / Graylog-specific
- `threads` (int) — Worker threads to process messages.
- `override_source` (string) — Override Graylog message `source` field value.
- `assign_all_partitions` (bool) — Assign all partitions explicitly (if available in your version).

### Security
- `security_protocol` (string) — `PLAINTEXT` | `SSL` | `SASL_PLAINTEXT` | `SASL_SSL`.
- `ssl_truststore_location` (string) — Path to truststore.
- `ssl_truststore_password` (string) — Truststore password.
- `ssl_keystore_location` (string) — Path to keystore.
- `ssl_keystore_password` (string) — Keystore password.
- `ssl_key_password` (string) — Private key password.
- `sasl_mechanism` (string) — e.g., `PLAIN`, `SCRAM-SHA-256`, `SCRAM-SHA-512`, `GSSAPI`.
- `sasl_username` (string) — SASL username (for PLAIN/SCRAM).
- `sasl_password` (string) — SASL password (for PLAIN/SCRAM).
- `sasl_jaas_config` (string) — Raw JAAS config string alternative to username/password.
- `sasl_kerberos_service_name` (string) — Kerberos service name (for GSSAPI).

### Minimal example (Kafka Raw)

```hcl
resource "graylog_input" "kafka_raw" {
  title  = "kafka-raw"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = {
    bootstrap_servers  = ["kafka:9092"]
    topics             = ["logs"]
    fetch_min_bytes    = 1
    group_id           = "graylog-kafka-raw"
    auto_offset_reset  = "latest"
  }
}
```

### Advanced example (SSL/SASL and tuning)

```hcl
resource "graylog_input" "kafka_raw_secure" {
  title  = "kafka-raw-secure"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = {
    bootstrap_servers       = ["kafka1:9093", "kafka2:9093"]
    topic_pattern           = "logs-.*"
    group_id                = "graylog-raw-secure"
    auto_offset_reset       = "earliest"
    max_poll_records        = 1000
    request_timeout_ms      = 30000
    fetch_min_bytes         = 1
    fetch_max_wait_ms       = 500
    max_partition_fetch_bytes = 1048576

    security_protocol       = "SASL_SSL"
    sasl_mechanism          = "SCRAM-SHA-512"
    sasl_username           = "user"
    sasl_password           = "pass"
    # Alternatively:
    # sasl_jaas_config      = "org.apache.kafka.common.security.scram.ScramLoginModule required username=\"user\" password=\"pass\";"

    # SSL options if needed by the cluster
    # ssl_truststore_location = "/etc/ssl/kafka/truststore.jks"
    # ssl_truststore_password = "changeit"

    threads                 = 2
    allow_auto_create_topics = false
  }
}
```

Notes:
- Not all keys are guaranteed to be available in every Graylog version; consult your Graylog/Kafka plugin documentation. Unknown keys will typically be ignored by the backend.
- Prefer `topics` for explicit lists or `topic_pattern`/`topic_filter` for dynamic selection; do not set them simultaneously.
- Values are passed as-is; ensure correct types (e.g., lists as Terraform lists, not comma-separated strings).
