---
page_title: "graylog_views Data Source - Graylog"
description: |-
  Lists Graylog Views and provides both full item objects and a title→id map for convenient lookups.
---

# graylog_views (Data Source)

Lists Graylog Views and returns:
- `items` — list of view objects (`id`, `title`, `description`).
- `title_map` — convenience map `title -> id` (exact titles; if duplicates exist, the last one wins).

Note: Some images/versions may not expose Views APIs. This data source will return an empty list in such cases.

## Example Usage

```hcl
data "graylog_views" "all" {}

output "views_map" {
  value = data.graylog_views.all.title_map
}

output "first_view" {
  value = try(data.graylog_views.all.items[0], null)
}
```

## Attributes Reference

- `items` (List(Object)) — list of objects with fields:
  - `id` (String)
  - `title` (String)
  - `description` (String)
- `title_map` (Map(String)) — `title -> id` map.
