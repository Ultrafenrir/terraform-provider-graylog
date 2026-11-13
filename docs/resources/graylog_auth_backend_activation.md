---
page_title: "graylog_auth_backend_activation Resource - Graylog Terraform Provider"
subcategory: "Users & Security"
description: |-
  Selects the active authentication backend for a Graylog cluster.
---

# graylog_auth_backend_activation (Resource)

Selects the cluster's active authentication backend, via `/system/authentication/services/configuration`.

Graylog separates *defining* a backend from *using* it: a [graylog_auth_backend](graylog_auth_backend) can exist without being active, and exactly one backend is active for the whole cluster at a time.

## Example Usage

```hcl
resource "graylog_auth_backend_activation" "active" {
  backend_id = graylog_auth_backend.ldap.id
}
```

## Singleton

The active backend is one cluster-wide setting, so this resource is a singleton and its `id` is always `active`. Declaring it twice means two resources fighting over the same setting, and each apply will flip it to whichever ran last — Terraform cannot detect the conflict for you.

Destroying the resource clears the selection and returns the cluster to local authentication only. The backend itself is left in place.

If something outside Terraform has since activated a different backend, destroy leaves that selection alone rather than clearing someone else's choice.

## Argument Reference

- `backend_id` (String, Required) — ID of the backend to activate.
- `timeouts` (Block, Optional) — `create`, `update` and `delete` timeouts.

## Attribute Reference

- `id` (String) — Always `active`.

## Import

```shell
terraform import graylog_auth_backend_activation.active active
```
