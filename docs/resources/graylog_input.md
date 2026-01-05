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
