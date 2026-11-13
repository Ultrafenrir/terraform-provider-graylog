---
page_title: "graylog_stream_output_binding Resource"
subcategory: "Streams"
description: |-
  Manages an attachment (binding) of a Graylog Output to a Stream.
---

# graylog_stream_output_binding (Resource)

Attaches an existing output to an existing stream. Use this resource to manage the relationship independently from `graylog_stream` and `graylog_output`.

Diff-aware behavior:
- Create attaches the output only when the relationship does not already exist.
- Update attaches the new `stream_id`/`output_id` pair before removing the old relationship, if it still exists.
- Delete detaches only an existing relationship and is a no-op when it was removed outside Terraform.

## Example Usage

```hcl
resource "graylog_stream_output_binding" "example" {
  stream_id = graylog_stream.s.id
  output_id = graylog_output.o.id
}
```

## Argument Reference

- `stream_id` (String, Required) — ID of the stream receiving the output.
- `output_id` (String, Required) — ID of the output attached to the stream.

## Attribute Reference

- `id` (String) — Synthetic ID in `stream_id/output_id` format.

## Import

Import an existing relationship with `stream_id/output_id` or `stream_id:output_id`:

```bash
terraform import graylog_stream_output_binding.example <stream_id>/<output_id>
# or
terraform import graylog_stream_output_binding.example <stream_id>:<output_id>
```

An identifier containing only one resource ID is rejected. If the relationship is removed outside Terraform, the next refresh removes this resource from state.
