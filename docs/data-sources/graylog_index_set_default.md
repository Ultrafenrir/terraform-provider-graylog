---
page_title: "graylog_index_set_default Data Source - Graylog Terraform Provider"
subcategory: "Index Sets"
description: |-
  Fetches the default (writable) Graylog index set.
---

# graylog_index_set_default (Data Source)

Fetches the default (writable) Graylog index set.

The provider lists every index set (`GET /api/system/indices/index_sets`) and returns the one flagged `default`. If no index set is flagged, the read fails with `Default index set not found`.

Use it wherever an API wants an index set **id** and you do not manage the index set in Terraform, such as `index_set_id` on [graylog_stream](../resources/graylog_stream).

## Example Usage

```hcl
data "graylog_index_set_default" "this" {}

resource "graylog_stream" "errors" {
  title        = "errors"
  index_set_id = data.graylog_index_set_default.this.id
}
```

## Argument Reference

This data source takes no arguments.

## Attributes Reference

- `id` (String) — The unique identifier of the default index set.
- `title` (String) — Title.
- `description` (String) — Description.
- `index_prefix` (String) — Index prefix.
- `shards` (Number) — Number of shards.
- `replicas` (Number) — Number of replicas.
- `index_analyzer` (String) — Index analyzer.
- `default` (Bool) — Whether this is the default index set. Always `true` here, since that is the set this data source selects.
