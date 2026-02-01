---
page_title: "graylog_pipeline Resource - Graylog Terraform Provider"
description: |-
  Terraform Graylog provider: manage classic Graylog pipelines (DSL in `source`) for automation/IaC. Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_pipeline (Resource)

Manages a classic Graylog pipeline. Part of the Graylog Terraform Provider for Graylog automation. Provide the entire pipeline DSL in the `source` attribute or manage only metadata (title/description).

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
