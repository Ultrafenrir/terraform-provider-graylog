---
page_title: "graylog_index_set_field_type_profile Resource - Graylog Terraform Provider"
subcategory: "Index Sets"
description: |-
  Manages a Graylog index set field type profile and its custom field mappings.
---

# graylog_index_set_field_type_profile (Resource)

Manages a reusable field type profile. Attach it to one or more index sets using
`graylog_index_set.field_type_profile`. Profile changes and binding changes update
configuration in place. They apply to indices created afterward; the provider
**does not rotate indices** or rewrite existing data. If new messages need the
changed mappings immediately, rotate the affected index sets separately in Graylog.

**Deleting a profile detaches it from every index set that uses it, including
index sets outside Terraform.** This is Graylog's own delete behavior, rather than
a refusal to delete an in-use profile. To keep a profile and its bindings when
stopping Terraform management, remove it from Terraform state instead of destroying
it. A resource reference ensures that Terraform normally destroys an index set
before its profile.

The REST contract is verified against Graylog **7.2.2**. Graylog 5 does not support
this resource. On other releases, the server must expose
`/system/indices/index_sets/profiles` and the index-set `field_type_profile` field.
Credentials need `mappingprofiles:create`, `mappingprofiles:read`,
`mappingprofiles:edit`, and `mappingprofiles:delete` for the corresponding
operations. Binding also requires index-set create/edit permissions.

## Example Usage

```hcl
resource "graylog_index_set_field_type_profile" "application" {
  name        = "Application fields"
  description = "Shared application field types"
  custom_field_mappings = {
    "client.ip"   = "ip"
    response_code = "long"
    duration_ms   = "double"
    message_text  = "string_fts"
  }
}

resource "graylog_index_set" "application" {
  title              = "Application logs"
  index_prefix       = "application"
  field_type_profile = graylog_index_set_field_type_profile.application.id
}
```

The mapping keys are field names; values are Graylog's logical types. For example,
use `string` for an aggregatable string, `string_fts` for a full-text searchable
string, and `geo-point` for a geographic point. Elasticsearch/OpenSearch physical
type names such as `keyword`, `text`, or `geo_point` are not accepted here.
Graylog validates protected built-in fields and rejects mappings that cannot be
changed. An empty map (`custom_field_mappings = {}`) clears the custom mappings.

## Argument Reference

- `name` (String, Required) — Profile name. Can be changed in place.
- `description` (String, Optional) — Description. Defaults to `""`; set `""` to clear it.
- `custom_field_mappings` (Map of String, Required) — Field-to-type mappings.
  Supported values are `string`, `string_fts`, `long`, `double`, `date`, `boolean`,
  `binary`, `geo-point`, and `ip`. Dotted field names are supported.

## Attributes Reference

- `id` (String) — Profile's Mongo ObjectID.

## Binding and Detaching

On `graylog_index_set`, `field_type_profile` has three distinct meanings:

| Configuration | Behavior |
| --- | --- |
| A profile ID or resource reference | Attach/switch the profile; detect and correct binding drift. |
| `""` | Detach the profile, sending JSON `null` to Graylog. |
| Omitted or `null` | Leave the server binding unmanaged and preserve it during unrelated updates. Removing the attribute does not detach the profile. |

An existing profile ID can be used directly without managing the profile resource.
Importing an index set adopts its current binding, if present. Include that
binding in configuration to manage it, or omit the attribute to leave it unmanaged.

## Import

Import a profile by ID:

```shell
terraform import graylog_index_set_field_type_profile.application <profile_id>
```

Import reads the current name, description, and all custom mappings. Copy them
into configuration to obtain an empty plan.
