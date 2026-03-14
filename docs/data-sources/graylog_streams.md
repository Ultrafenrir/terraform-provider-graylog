---
page_title: "graylog_streams Data Source - Graylog"
description: |-
  Lists Graylog streams and provides both full item objects and a title→id map for convenient lookups.
---

# graylog_streams (Data Source)

Lists Graylog streams and returns:
- `items` — list of stream objects (`id`, `title`, `description`, `disabled`, `index_set_id`).
- `title_map` — convenience map `title -> id` (exact titles; if duplicates exist, the last one wins).

## Example Usage

```hcl
data "graylog_streams" "all" {}

output "streams_map" {
  value = data.graylog_streams.all.title_map
}

output "first_stream" {
  value = try(data.graylog_streams.all.items[0], null)
}
```

## Attributes Reference

- `items` (List(Object)) — list of objects with fields:
  - `id` (String)
  - `title` (String)
  - `description` (String)
  - `disabled` (Bool)
  - `index_set_id` (String)
- `title_map` (Map(String)) — `title -> id` map.
