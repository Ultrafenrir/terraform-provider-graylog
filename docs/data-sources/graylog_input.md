---
page_title: "graylog_input Data Source - Graylog"
description: |-
  Retrieves a Graylog input by filters (e.g., by title).
---

# graylog_input (Data Source)

Retrieve information about a Graylog input.

## Example Usage

```hcl
data "graylog_input" "by_title" {
  title = "kafka-json"
}

output "input_id" {
  value = data.graylog_input.by_title.id
}
```

## Argument Reference

- `title` (String, Optional) — Filter by input title.

## Attributes Reference

- `id` — Input ID.
- `title` — Title.
- Other fields may be exposed depending on provider implementation.
