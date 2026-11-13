---
page_title: "graylog_lookup_adapter Resource - Graylog Terraform Provider"
subcategory: "Lookup Tables"
description: |-
  Manages a Graylog lookup data adapter.
---

# graylog_lookup_adapter (Resource)

Manages a Graylog lookup data adapter, the backing data source of a [lookup table](graylog_lookup_table).

## Example Usage

```hcl
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

resource "graylog_lookup_adapter" "tenants" {
  name  = "tenant-names"
  title = "Tenant names"

  config_json = jsonencode({
    type                    = "csvfile"
    path                    = "/etc/graylog/server/tenants.csv"
    separator               = ","
    quotechar               = "\""
    key_column              = "tenant_id"
    value_column            = "tenant_name"
    check_interval          = 60
    case_insensitive_lookup = false
  })
}
```

## About `config_json`

Adapter configuration is polymorphic: each adapter type (`csvfile`, `maxmind_geoip`, `httpjsonpath`, `dnslookup`, …) carries its own field set, so it is expressed as JSON rather than as typed attributes. The `type` discriminator belongs inside the document.

Graylog fills in the defaults of the chosen type when it stores the configuration. Only the keys present in your configuration take part in drift detection, so those additions never show up as a diff. A key you *do* manage changing server-side is still reported.

The attribute is marked sensitive, because adapter configurations routinely carry credentials — HTTP headers with API tokens, MaxMind licence keys, S3 secrets.

To see the field set a type accepts:

```shell
curl -u admin:<password> http://graylog.example.com/api/system/lookup/types/adapters
```

## Argument Reference

- `name` (String, Required) — Unique name. Renaming is an in-place update.
- `title` (String, Required) — Human readable title.
- `config_json` (String, Required, Sensitive) — Adapter configuration, JSON-encoded, including `type`.
- `description` (String, Optional) — Adapter description.
- `timeouts` (Block, Optional) — `create`, `update` and `delete` timeouts.

## Attribute Reference

- `id` (String) — Lookup data adapter ID.

## Import

```shell
terraform import graylog_lookup_adapter.geoip_city geoip-city
```

Both the ID and the name are accepted; the ID is what ends up in state. Import has no prior document to compare against, so it stores the server's full configuration including type defaults; the first plan afterwards may show that difference once.
