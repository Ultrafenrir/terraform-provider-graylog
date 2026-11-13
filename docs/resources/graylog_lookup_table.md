---
page_title: "graylog_lookup_table Resource - Graylog Terraform Provider"
subcategory: "Lookup Tables"
description: |-
  Manages a Graylog lookup table and its cache and data adapter bindings.
---

# graylog_lookup_table (Resource)

Manages a Graylog lookup table, which binds a [cache](graylog_lookup_cache) and a [data adapter](graylog_lookup_adapter) under the name that pipeline rules resolve with `lookup()` and `lookup_value()`.

## Example Usage

A GeoIP enrichment stack, referenced from a pipeline rule:

```hcl
resource "graylog_lookup_cache" "geoip" {
  name  = "geoip-cache"
  title = "GeoIP Cache"

  config_json = jsonencode({
    type                     = "guava_cache"
    max_size                 = 1000
    expire_after_access      = 60
    expire_after_access_unit = "SECONDS"
    expire_after_write       = 0
  })
}

resource "graylog_lookup_adapter" "geoip_city" {
  name  = "geoip-city"
  title = "GeoIP City"

  config_json = jsonencode({
    type                = "maxmind_geoip"
    path                = "/usr/share/graylog/data/geolocation/GeoLite2-City.mmdb"
    database_type       = "MAXMIND_CITY"
    check_interval      = 1
    check_interval_unit = "MINUTES"
  })
}

resource "graylog_lookup_table" "geoip_city" {
  name            = "geoip-city-lookup"
  title           = "GeoIP City Lookup"
  cache_id        = graylog_lookup_cache.geoip.id
  data_adapter_id = graylog_lookup_adapter.geoip_city.id
}

resource "graylog_pipeline_rule" "geoip" {
  source = <<-EOT
    rule "GeoIP Lookup"
    when
      has_field("src_ip")
    then
      let geo = lookup("geoip-city-lookup", to_string($message.src_ip));
      set_field("src_ip_geo_country", geo["country"].iso_code);
    end
  EOT

  depends_on = [graylog_lookup_table.geoip_city]
}
```

Referencing `graylog_lookup_cache.geoip.id` rather than a literal ID is what lets Terraform create the table after its dependencies and destroy it before them. Graylog refuses to delete a cache or adapter that a table still references, so a literal ID turns an ordering mistake into a failed destroy.

## Argument Reference

- `name` (String, Required) — Unique name. This is the string pipeline rules pass to `lookup()`, so renaming a table stops every rule that referenced the old name from resolving.
- `title` (String, Required) — Human readable title.
- `cache_id` (String, Required) — ID of the cache to use.
- `data_adapter_id` (String, Required) — ID of the data adapter to use.
- `description` (String, Optional) — Lookup table description.
- `default_single_value` (String, Optional) — Value returned for a single-value lookup that finds nothing.
- `default_single_value_type` (String, Optional) — One of `STRING`, `NUMBER`, `BOOLEAN`, `OBJECT`, `NULL`.
- `default_multi_value` (String, Optional) — Value returned for a multi-value lookup that finds nothing.
- `default_multi_value_type` (String, Optional) — Graylog only accepts `OBJECT` or `NULL`.
- `timeouts` (Block, Optional) — `create`, `update` and `delete` timeouts.

The four default-value attributes are also computed: leave them out and the server's own values (`""` / `NULL`) are adopted instead of being fought over on every plan.

## Attribute Reference

- `id` (String) — Lookup table ID.

## Import

```shell
terraform import graylog_lookup_table.geoip_city geoip-city-lookup
```

Both the ID and the name are accepted; the ID is what ends up in state.
