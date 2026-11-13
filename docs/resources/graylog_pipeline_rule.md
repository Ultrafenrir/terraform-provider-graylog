---
page_title: "graylog_pipeline_rule Resource - Graylog Terraform Provider"
subcategory: "Pipelines"
description: |-
  Manages a Graylog pipeline rule.
---

# graylog_pipeline_rule (Resource)

Manages a Graylog pipeline rule. Pipelines reference rules by the name declared in the rule source, so create rules with this resource and reference them from the `source` of a `graylog_pipeline`.

## Example Usage

```hcl
resource "graylog_pipeline_rule" "drop_debug" {
  description = "Drop debug messages"
  source      = <<-EOT
    rule "drop debug"
    when
      to_long($message.level) > 6
    then
      drop_message();
    end
  EOT
}

resource "graylog_pipeline" "default" {
  title  = "default"
  source = <<-EOT
    pipeline "default"
    stage 0 match either
      rule "drop debug";
    end
  EOT

  depends_on = [graylog_pipeline_rule.drop_debug]
}
```

## Argument Reference

- `source` (String, Required) — Rule source in the pipeline rule DSL (`rule "name" when ... then ... end`).
- `description` (String, Optional) — Rule description.

## Attribute Reference

- `id` (String) — Pipeline rule ID.
- `title` (String) — Rule title; Graylog derives it from the rule name declared in `source`.

## Import

```shell
terraform import graylog_pipeline_rule.drop_debug <rule-id>
```
