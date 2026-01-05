---
page_title: "graylog_index_set Data Source - Graylog"
description: |-
  Retrieves a Graylog index set by filters (e.g., by title).
---

# graylog_index_set (Data Source)

Retrieve information about a Graylog index set.

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
