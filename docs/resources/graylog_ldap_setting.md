---
page_title: "graylog_ldap_setting Resource - Graylog Terraform Provider"
subcategory: "Users & Security"
description: |-
  Manages legacy global LDAP settings in Graylog as a singleton resource.
---

# graylog_ldap_setting (Resource)

> **Deprecated.** This resource targets the pre-4.0 `/system/ldap/settings` endpoint, which Graylog no longer serves. Use [graylog_auth_backend](graylog_auth_backend) together with [graylog_auth_backend_activation](graylog_auth_backend_activation) instead.

Manages the legacy global LDAP settings in Graylog. This is a singleton resource.

## Example Usage

```hcl
resource "graylog_ldap_setting" "this" {
  enabled       = false
  ldap_uri      = "ldap://ldap.example.org:389"
  search_base   = "dc=example,dc=org"
  search_pattern = "(uid={0})"
}
```

## Argument Reference

- `enabled` (Boolean, Optional) — Enables LDAP authentication.
- `system_username` (String, Optional) — Bind DN or system username.
- `system_password` (String, Optional, Sensitive) — Bind password.
- `ldap_uri` (String, Optional) — LDAP server URI.
- `search_base` (String, Optional) — Base DN for user searches.
- `search_pattern` (String, Optional) — User search pattern.
- `user_unique_id_attribute` (String, Optional) — Attribute containing the unique user identifier.
- `group_search_base` (String, Optional) — Base DN for group searches.
- `group_search_pattern` (String, Optional) — Group search pattern.
- `default_group` (String, Optional) — Default Graylog group.
- `use_start_tls` (Boolean, Optional) — Enables STARTTLS.
- `trust_all_certificates` (Boolean, Optional) — Trusts all LDAP server certificates.
- `active_directory` (Boolean, Optional) — Enables Active Directory mode.
- `display_name_attribute` (String, Optional) — Attribute containing the display name.
- `email_attribute` (String, Optional) — Attribute containing the email address.

## Attributes Reference

- `id` (String) — Fixed to `ldap`.

## Import

```shell
terraform import graylog_ldap_setting.this ldap
```
