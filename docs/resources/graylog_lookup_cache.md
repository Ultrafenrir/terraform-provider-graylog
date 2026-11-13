---
page_title: "graylog_lookup_cache Resource - Graylog Terraform Provider"
subcategory: "Lookup Tables"
description: |-
  Manages a Graylog lookup cache.
---

# graylog_lookup_cache (Resource)

Manages a Graylog lookup cache, the caching half of a [lookup table](graylog_lookup_table).

## Example Usage

```hcl
resource "graylog_lookup_cache" "geoip" {
  name        = "geoip-cache"
  title       = "GeoIP Cache"
  description = "Node-local, in-memory cache"

  config_json = jsonencode({
    type                     = "guava_cache"
    max_size                 = 1000
    expire_after_access      = 60
    expire_after_access_unit = "SECONDS"
    expire_after_write       = 0
  })
}
```

## About `config_json`

Cache configuration is polymorphic: each cache type carries its own field set, so it is expressed as JSON rather than as typed attributes. The `type` discriminator belongs inside the document.

Graylog fills in the defaults of the chosen type when it stores the configuration — submitting five keys to a `guava_cache` reads back nine, the extra four being nulls you never wrote. Only the keys present in your configuration take part in drift detection, so those additions never show up as a diff. A key you *do* manage changing server-side is still reported.

To see the field set a type accepts:

```shell
curl -u admin:<password> http://graylog.example.com/api/system/lookup/types/caches
```

## Argument Reference

- `name` (String, Required) — Unique name. Renaming is an in-place update.
- `title` (String, Required) — Human readable title.
- `config_json` (String, Required) — Cache configuration, JSON-encoded, including `type`.
- `description` (String, Optional) — Cache description.
- `timeouts` (Block, Optional) — `create`, `update` and `delete` timeouts.

## Attribute Reference

- `id` (String) — Lookup cache ID.

## Import

```shell
terraform import graylog_lookup_cache.geoip geoip-cache
```

Both the ID and the name are accepted; the ID is what ends up in state. Import has no prior document to compare against, so it stores the server's full configuration including type defaults; the first plan afterwards may show that difference once.
