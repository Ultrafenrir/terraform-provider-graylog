---
page_title: "graylog_inputs Data Source - Graylog"
description: |-
  Lists Graylog inputs and provides both full item objects and a title→id map for convenient lookups.
---

# graylog_inputs (Data Source)

Lists Graylog inputs and returns:
- `items` — list of input objects (`id`, `title`, `type`, `global`, `node`).
- `title_map` — convenience map `title -> id` (exact titles; if duplicates exist, the last one wins).

Note: depending on Graylog version, `id` is available for inputs created via API. For built‑in inputs, `id` may be empty.

## Example Usage

```hcl
data "graylog_inputs" "all" {}

output "inputs_map" {
  value = data.graylog_inputs.all.title_map
}

output "first_input" {
  value = try(data.graylog_inputs.all.items[0], null)
}
```

## Attributes Reference

- `items` (List(Object)) — list of objects with fields:
  - `id` (String)
  - `title` (String)
  - `type` (String)
  - `global` (Bool)
  - `node` (String)
- `title_map` (Map(String)) — `title -> id` map.
