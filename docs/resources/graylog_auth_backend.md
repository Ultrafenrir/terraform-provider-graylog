# graylog_auth_backend (Resource)

Manages a Graylog authentication service backend — LDAP or Active Directory — via `/system/authentication/services/backends`.

Creating a backend does **not** activate it. Graylog keeps the active selection as a separate cluster-wide setting; see [graylog_auth_backend_activation](graylog_auth_backend_activation).

This supersedes `graylog_ldap_setting`, which targets the pre-4.0 `/system/ldap/settings` endpoint that no longer exists.

## Example Usage

```hcl
data "graylog_role" "reader" {
  name = "Reader"
}

resource "graylog_auth_backend" "ldap" {
  title       = "Corporate LDAP"
  description = "Directory-backed logins"

  default_roles = [data.graylog_role.reader.id]

  system_user_password = var.ldap_bind_password

  config_json = jsonencode({
    type                     = "ldap"
    servers                  = [{ host = "ldap.example.com", port = 389 }]
    transport_security       = "none"
    verify_certificates      = false
    system_user_dn           = "cn=admin,dc=example,dc=org"
    user_full_name_attribute = "cn"
    user_name_attribute      = "uid"
    user_search_base         = "dc=example,dc=org"
    user_search_pattern      = "(&(uid={0})(objectClass=person))"
    user_unique_id_attribute = "entryUUID"
  })
}

resource "graylog_auth_backend_activation" "active" {
  backend_id = graylog_auth_backend.ldap.id
}
```

For Active Directory set `type = "active-directory"` and the attribute names it uses — typically `sAMAccountName` for `user_name_attribute` and `objectGUID` for `user_unique_id_attribute`.

## The write-only password

`system_user_password` is its own attribute rather than a key inside `config_json`, because it is the one field that cannot round-trip: Graylog stores it and then reports `{"is_set": true}` instead of the value. Putting it in the schema makes that visible where it matters.

- It is never refreshed. A password rotated directly in Graylog is invisible to Terraform, and the value in state is the only record of what was applied.
- Omitting the attribute keeps whatever password is already stored. That is what makes an imported backend, or one whose secret was set elsewhere, manageable from here. An empty string is rejected at plan time rather than quietly meaning the same thing.
- Setting it inside `config_json` is rejected at apply time; two sources of truth for the same field would silently disagree.

Graylog also adds fields of its own to a stored configuration — `email_attributes` is the common one. Only the keys present in your `config_json` take part in drift detection, so those additions never surface as a diff.

## Argument Reference

- `title` (String, Required) — Backend title.
- `config_json` (String, Required) — Backend configuration, JSON-encoded, including `type`. The bind password does not belong here.
- `system_user_password` (String, Optional, Sensitive) — Password for the system user that binds to the directory. Write-only; omit it to keep the stored one. Must be non-empty when set.
- `description` (String, Optional) — Backend description.
- `default_roles` (Set of String, Optional) — Role IDs granted to every user this backend authenticates. Unordered.
- `timeouts` (Block, Optional) — `create`, `update` and `delete` timeouts.

## Attribute Reference

- `id` (String) — Authentication backend ID.

## Destroying

Graylog refuses to delete an authentication backend while anything still
references it, and there are two such references:

- the backend is the cluster's active one — destroy
  `graylog_auth_backend_activation` first, which Terraform does on its own
  when the activation refers to the backend by attribute;
- user profiles created by it still exist. Every user who has logged in
  through the backend holds one, and they are not managed by Terraform, so
  `terraform destroy` fails with `<id> is still in use` until they are
  removed.

The provider does not delete those profiles for you: they carry roles,
dashboards and saved searches, and removing them to satisfy a destroy would
be a surprising amount of collateral damage.

## Import

```shell
terraform import graylog_auth_backend.ldap 6a88d44aa65258664d841d41
```

Import adopts the server's configuration, including fields Graylog added on its own — the first plan afterwards narrows it to the keys you actually declare. The password is not part of it, so add `system_user_password` to your configuration before the next apply; until then the stored one stays in place.
