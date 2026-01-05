---
page_title: "graylog_pipeline Resource - Graylog"
description: |-
  Manages a classic Graylog pipeline. The full pipeline definition can be provided via the `source` attribute.
---

# graylog_pipeline (Resource)

Manages a classic Graylog pipeline. Provide the entire pipeline DSL in the `source` attribute or manage only metadata (title/description).

## Example Usage

```hcl
resource "graylog_pipeline" "sanitize" {
  title       = "sanitize"
  description = "Drop empty messages"
  source = <<-EOT
    pipeline "sanitize"
    stage 0 match either
    rule "drop_empty";

    rule "drop_empty"
    when to_string($message.message) == ""
    then drop_message();
    end
  EOT
}
```

## Argument Reference

- `title` (String, Required) — Pipeline title.
- `description` (String, Optional) — Description.
- `source` (String, Optional) — Full pipeline source (DSL).
- `timeouts` (Block, Optional) — Customize create/update/delete timeouts.

## Attributes Reference

- `id` — Pipeline ID.

## Import

```bash
terraform import graylog_pipeline.p <pipeline_id>
```
