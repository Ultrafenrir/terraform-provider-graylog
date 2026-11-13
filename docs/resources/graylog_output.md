---
page_title: "graylog_output Resource - Graylog Terraform Provider"
subcategory: "Inputs & Outputs"
description: |-
  Manages a Graylog output and its stream attachments.
---

# graylog_output (Resource)

Creates or updates a Graylog output and optionally attaches it to one or more streams.

## Example Usage

```hcl
data "graylog_index_set_default" "this" {}

resource "graylog_stream" "s" {
  title        = "out-stream"
  description  = "Stream for outputs"
  index_set_id = data.graylog_index_set_default.this.id
}

resource "graylog_output" "gelf" {
  title = "to-local-gelf"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
  streams = [graylog_stream.s.id]
}
```

## Argument Reference

- `title` (String, Required) — Output title.
- `type` (String, Required) — Fully qualified Graylog output class, such as `org.graylog2.outputs.GelfOutput`.
- `configuration` (String, Optional) — JSON-encoded output configuration, typically produced with `jsonencode`.
- `streams` (List of String, Optional) — Stream IDs to which the output is attached.

## Attributes Reference

- `id` (String) — Output ID.

## Import

```shell
terraform import graylog_output.gelf <output_id>
```
