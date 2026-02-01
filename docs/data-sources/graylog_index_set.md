---
page_title: "graylog_index_set Data Source - Graylog Terraform Provider"
description: |-
  Terraform Graylog provider: retrieve Graylog index sets by filters (e.g., by title) for automation/IaC. Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_index_set (Data Source)

Retrieve information about a Graylog index set. Part of the Graylog Terraform Provider for Graylog automation.

## Example Usage

```hcl
data "graylog_index_set" "by_title" {
  title = "main-index"
}

output "index_set_id" {
  value = data.graylog_index_set.by_title.id
}
```

## Argument Reference

- `title` (String, Optional) — Filter by index set title.

## Attributes Reference

- `id` — Index set ID.
- `title` — Title.
- Other fields may be exposed depending on provider implementation.
