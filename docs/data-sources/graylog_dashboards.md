---
page_title: "graylog_dashboards Data Source - Graylog"
description: |-
  Lists Graylog classic dashboards and provides both full item objects and a title→id map.
---

# graylog_dashboards (Data Source)

Lists classic Graylog dashboards and returns:
- `items` — list of objects (`id`, `title`, `description`).
- `title_map` — convenience map `title -> id`.

## Example Usage

```hcl
data "graylog_dashboards" "all" {}

output "dashboards_map" {
  value = data.graylog_dashboards.all.title_map
}
```

## Attributes Reference

- `items` (List(Object)) — fields:
  - `id` (String)
  - `title` (String)
  - `description` (String)
- `title_map` (Map(String)) — `title -> id` map.
