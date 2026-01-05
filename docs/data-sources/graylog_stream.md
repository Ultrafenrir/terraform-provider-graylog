---
page_title: "graylog_stream Data Source - Graylog"
description: |-
  Retrieves a Graylog stream by filters (e.g., by title).
---

# graylog_stream (Data Source)

Retrieve information about a Graylog stream.

## Example Usage

```hcl
data "graylog_stream" "by_title" {
  title = "errors"
}

output "stream_id" {
  value = data.graylog_stream.by_title.id
}
```

## Argument Reference

- `title` (String, Optional) — Filter by stream title.

## Attributes Reference

- `id` — Stream ID.
- `title` — Title.
- Other fields may be exposed depending on provider implementation.
