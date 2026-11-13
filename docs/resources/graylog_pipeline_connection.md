---
page_title: "graylog_pipeline_connection Resource - Graylog Terraform Provider"
subcategory: "Pipelines"
description: |-
  Manages the pipelines connected to a Graylog stream.
---

# graylog_pipeline_connection (Resource)

Manages the set of pipelines connected to a stream. The resource owns the **full** connection list for its stream: applying it replaces whatever pipelines were connected to that stream before, and destroying it disconnects all pipelines from the stream.

Use at most one `graylog_pipeline_connection` per stream.

## Example Usage

```hcl
data "graylog_stream" "all_messages" {
  title = "Default Stream"
}

resource "graylog_pipeline_connection" "default" {
  stream_id    = data.graylog_stream.all_messages.id
  pipeline_ids = [graylog_pipeline.default.id]
}
```

## Argument Reference

- `stream_id` (String, Required, Forces new resource) — Stream ID to connect pipelines to.
- `pipeline_ids` (Set of String, Required) — Pipeline IDs connected to the stream.

## Attribute Reference

- `id` (String) — Same as `stream_id`.

## Import

```shell
terraform import graylog_pipeline_connection.default <stream-id>
```
