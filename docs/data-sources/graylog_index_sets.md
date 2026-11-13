---
page_title: "graylog_index_sets Data Source - Graylog"
subcategory: "Index Sets"
description: |-
  Lists Graylog index sets and provides both full item objects and a title→id map.
---

# graylog_index_sets (Data Source)

Lists Graylog index sets and returns:
- `items` — list of objects (`id`, `title`, `description`, `index_prefix`, `default`).
- `title_map` — convenience map `title -> id`.

## Example Usage

```hcl
data "graylog_index_sets" "all" {}

output "index_sets_map" {
  value = data.graylog_index_sets.all.title_map
}
```

## Attributes Reference

- `items` (List(Object)) — fields:
  - `id` (String)
  - `title` (String)
  - `description` (String)
  - `index_prefix` (String)
  - `default` (Bool)
- `title_map` (Map(String)) — `title -> id` map.
