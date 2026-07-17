---
page_title: "graylog_input Resource - Graylog Terraform Provider"
subcategory: "Inputs & Outputs"
description: |-
  Terraform Graylog provider: manage Graylog inputs (flexible configuration, extractors) for automation/IaC. Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_input (Resource)

Manages a Graylog input. Part of the Graylog Terraform Provider for Graylog automation. Compatible with Graylog v5/v6/v7. The `configuration` attribute is passed as JSON string (use `jsonencode({...})`) supporting strings, numbers, booleans, lists and nested objects, covering all input types (including Kafka inputs). Extractors are managed as structured `extractor` blocks.

## Example Usage

```hcl
resource "graylog_input" "kafka_raw" {
  title  = "kafka-raw"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = jsonencode({
    legacy_mode      = false # required to use bootstrap_server below instead of ZooKeeper
    bootstrap_server = "localhost:9092"
    topic_filter     = "^logs-.*$" # regex
    fetch_min_bytes  = 1
    fetch_wait_max   = 100
    threads          = 2
    group_id         = "graylog-kafka-raw"
  })

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
```

## Argument Reference

- `title` (String, Required) — Input title.
- `type` (String, Required) — Fully qualified input class (e.g. `org.graylog2.inputs.syslog.udp.SyslogUDPInput`).
- `global` (Boolean, Optional, Computed) — Whether the input is global. Defaults to `false`.
- `node` (String, Optional) — Node ID to run the input on when not global.
- `configuration` (String(JSON), Optional, **Sensitive**) — JSON-encoded configuration object. Values may be strings, numbers, booleans, lists, or nested objects. Marked sensitive because many input types (notably Kafka's `custom_properties`, see below) embed credentials directly in this blob; the whole attribute is hidden in `terraform plan`/`apply` output as a result.
- `extractor` (Block, Optional, repeatable) — An extractor attached to this input. Extractors are reconciled by identity on update: unchanged extractors are left alone, only added/removed/changed ones are created or deleted.
  - `title` (String, Required) — Extractor title.
  - `extractor_type` (String, Required) — e.g. `regex`, `grok`, `substring`, `split_and_index`, `copy_input`, `regex_replace`, `json`, `lookup_table` (availability depends on Graylog version/plugins).
  - `source_field` (String, Required) — Message field to read from.
  - `target_field` (String, Optional) — Message field to write the extracted value to.
  - `cursor_strategy` (String, Optional, Computed) — `copy` (leave the source field untouched) or `cut` (remove the matched part). Defaults to `copy`.
  - `extractor_config` (String(JSON), Optional) — JSON-encoded extractor-type-specific configuration (e.g. `{"regex_value": "..."}` for `regex`, `{"grok_pattern": "..."}` for `grok`).
  - `condition_type` (String, Optional, Computed) — `none` (always run), `string` (source field contains `condition_value`), or `regex` (source field matches `condition_value`). Defaults to `none`.
  - `condition_value` (String, Optional) — Required when `condition_type` is not `none`.
  - `order` (Number, Optional, Computed) — Execution order; if omitted, Graylog assigns the next available position.
  - `converter` (Block, Optional, repeatable) — Converters applied to the extracted value, in order.
    - `type` (String, Required) — e.g. `numeric`, `lowercase`, `uppercase`, `hash`, `date`, `csv`, `tokenizer`, `ip_anonymizer`, `splitandcount`, `syslog_pri`.
    - `config` (String(JSON), Optional) — JSON-encoded converter-specific configuration.
  - `id` (Computed) — Extractor ID.
- `timeouts` (Block, Optional) — Customize create/update/delete timeouts.

## Attributes Reference

- `id` — Input ID.

## Import

```bash
terraform import graylog_input.i <input_id>
```

---

## Kafka input configuration

Kafka Raw Input class: `org.graylog2.inputs.raw.kafka.RawKafkaInput`. Three other Kafka-backed
inputs (`org.graylog.plugins.cef.input.CEFKafkaInput`, `org.graylog2.inputs.syslog.kafka.SyslogKafkaInput`,
`org.graylog2.inputs.gelf.kafka.GELFKafkaInput`) share the exact same connection/`custom_properties`
shape below, plus their own codec-specific fields.

**These field names were verified live against Graylog 5.0.13, 6.0.14, and 7.0.10** via
`GET /api/system/inputs/types/org.graylog2.inputs.raw.kafka.RawKafkaInput` on each — the schema is
identical across all three (same fields, same requiredness), with one behavioral difference noted
below (`legacy_mode`'s default). An earlier version of this doc (and `examples/inputs/kafka_raw.tf`)
documented a different, incorrect key set (`bootstrap_servers` as a list, `topics`,
`security_protocol`, `ssl_truststore_location`, `sasl_mechanism`, etc.) that Graylog's native Kafka
input has never supported on any of these versions — those keys were silently ignored by the
backend and had no effect. If you used any of those keys, switch to the fields below.

### Connection / consumer
- `legacy_mode` (bool, optional) — `true` uses the old ZooKeeper-based consumer API (pre-Graylog 3.3) and ignores `bootstrap_server`/`custom_properties` entirely. **Always set this explicitly to `false`** to use the modern Kafka client — required for `bootstrap_server` and any SSL/SASL settings in `custom_properties` to take effect. Its default differs by version — `true` on Graylog 5.x/6.x, `false` on 7.x (verified live) — so don't rely on the default either way; set it explicitly. Forgetting this is the most common way for Kafka SSL/SASL configuration to appear to do nothing.
- `bootstrap_server` (string, optional) — Comma-separated list of brokers as a single string, e.g. `"host1:9092,host2:9092"` — **not** a Terraform list. Not used in legacy mode.
- `zookeeper` (string, optional) — ZooKeeper `host:port`. Only used in legacy mode.
- `topic_filter` (string, **required**) — Regular expression; every topic matching it is consumed. This is a regex, not a literal topic name or a glob.
- `fetch_min_bytes` (number, required) — Minimum batch size (bytes) to wait for before fetching.
- `fetch_wait_max` (number, required) — Max wait time (ms) for a batch to reach `fetch_min_bytes` before fetching anyway.
- `threads` (number, required) — Processor threads; use one per Kafka topic partition.
- `offset_reset` (string, optional, default `"largest"`) — `"largest"` (latest) or `"smallest"` (earliest) — what to do when there's no valid offset.
- `group_id` (string, optional, default `"graylog2"`) — Consumer group ID.
- `override_source` (string, optional) — Override the message `source` field.
- `charset_name` (string, optional, default `"UTF-8"`) — Message encoding.
- `throttling_allowed` (bool, optional, default `false`) — Pause reading from this input if Graylog can't keep up with message load.

### `custom_properties` — SSL/SASL, keystores, and anything else

There is no dedicated field per Kafka client property (no `ssl_truststore_location`,
`sasl_mechanism`, `security_protocol`, etc.). Instead, everything beyond the fields above goes
into a single `custom_properties` string: **newline-separated `key=value` pairs, using Kafka's
own dotted property names** (e.g. `ssl.truststore.location`, not `ssl_truststore_location`).
Graylog itself marks this field `is_sensitive` server-side — matching that, this provider marks
the whole `configuration` attribute `Sensitive` (see Argument Reference above), so none of this
appears in `terraform plan`/`apply` output.

```hcl
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
```

### Minimal example

```hcl
resource "graylog_input" "kafka_raw" {
  title  = "kafka-raw"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = jsonencode({
    legacy_mode      = false
    bootstrap_server = "kafka:9092"
    topic_filter     = "^logs-.*$"
    fetch_min_bytes  = 1
    fetch_wait_max   = 100
    threads          = 2
    group_id         = "graylog-kafka-raw"
    offset_reset     = "largest"
  })
}
```

### Secure example (SASL_SSL with keystore/truststore certs)

See `examples/inputs/kafka_raw.tf` for the full example with sensitive variables. In short:

```hcl
resource "graylog_input" "kafka_raw_secure" {
  title  = "kafka-raw-secure"
  type   = "org.graylog2.inputs.raw.kafka.RawKafkaInput"
  global = true

  configuration = jsonencode({
    legacy_mode      = false
    bootstrap_server = "kafka1:9093,kafka2:9093"
    topic_filter     = "^logs-.*$"
    fetch_min_bytes  = 1
    fetch_wait_max   = 500
    threads          = 2
    group_id         = "graylog-raw-secure"
    offset_reset     = "earliest"

    custom_properties = join("\n", [
      "security.protocol=SASL_SSL",
      "ssl.truststore.location=/etc/graylog/server/certs/kafka.truststore.jks",
      "ssl.truststore.password=${var.kafka_truststore_password}",
      "sasl.mechanism=PLAIN",
      "sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required username=\"${var.kafka_sasl_username}\" password=\"${var.kafka_sasl_password}\";",
    ])
  })
}
```

Notes:
- Field availability and behavior (create/update/read round-trip, including `custom_properties` and idempotency across `terraform plan`) were verified live against Graylog 5.0.13, 6.0.14, and 7.0.10. Only `legacy_mode`'s default differs between them (see above); double-check with `GET /api/system/inputs/types/<class>` against your own server if in doubt.
- `topic_filter` is a regex, evaluated against topic names — it is required even in legacy mode.
