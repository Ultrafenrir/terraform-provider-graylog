---
page_title: "graylog_stream Resource - Graylog Terraform Provider"
subcategory: "Streams"
description: |-
  Terraform Graylog provider: manage Graylog streams and rules (automation/IaC). Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation. Supports rule resynchronization on updates.
---

# graylog_stream (Resource)

Manages a Graylog stream. Part of the Graylog Terraform Provider for Graylog automation. Stream rules are managed via dedicated Graylog APIs and are resynchronized on updates (existing rules are removed and recreated to match the plan).

## Example Usage

```hcl
resource "graylog_stream" "errors" {
  title        = "errors"
  description  = "Error logs"
  index_set_id = graylog_index_set.main.id
  # Remove matching messages from the default stream
  remove_matches_from_default_stream = true

  rule {
    field       = "level"
    type        = 1      # equals/exact match (enum varies by Graylog version)
    value       = "ERROR"
    inverted    = false
    description = "Only error level"
  }

  rule {
    field       = "message"
    type        = 3      # regex match
    value       = ".*timeout.*"
    description = "Contains 'timeout'"
  }
}
```

## Argument Reference

- `title` (String, Required) — Stream title.
- `description` (String, Optional) — Stream description.
- `disabled` (Boolean, Optional) — Whether the stream is disabled.
- `index_set_id` (String, Optional) — Index set ID to use for the stream.
- `remove_matches_from_default_stream` (Boolean, Optional, Computed) — When true, messages matching this stream are removed from the default stream. If not set in configuration, the provider reads the current server value (defaults to `false`) and keeps it in state without causing diffs (including after import).
- `timeouts` (Block, Optional) — Customize create/update/delete timeouts.

### rule (Block)
- `id` (Computed) — Rule ID.
- `field` (String, Required) — Field to match.
- `type` (Int, Required) — Rule type (Graylog integer enum; value depends on Graylog version).
- `value` (String, Required) — Value to match.
- `inverted` (Boolean, Optional) — Invert the rule condition.
- `description` (String, Optional) — Rule description.

## Attributes Reference

- `id` — Stream ID.

## Import

You can import by ID (UUID/24-hex) or by exact title. For title-based import, use the explicit `title:` prefix. If multiple streams share the same title, import by ID.

```bash
# By ID
terraform import graylog_stream.s 5f3c2a0b9e0f1a2b3c4d5e6f

# By title (exact match)
terraform import graylog_stream.s "title:errors"
```
