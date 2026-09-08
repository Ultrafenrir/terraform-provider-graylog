---
page_title: "graylog_role Resource - Graylog Terraform Provider"
subcategory: "Users & Security"
description: |-
  Terraform Graylog provider: manage Graylog roles (automation/IaC). Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation. Управляет ролями в Graylog (название роли используется как идентификатор).
---

# graylog_role

Ресурс управляет пользовательскими ролями в Graylog: описанием и набором permissions.

## Example Usage

```
resource "graylog_role" "readonly" {
  name        = "tf-readonly"
  description = "Readonly role managed by Terraform"
  permissions = [
    "dashboards:read",
    "indices:read",
  ]
}
```

## Argument Reference

- `name` — (Required) Имя роли (immutable).
- `description` — (Optional) Описание.
- `permissions` — (Optional) Массив permissions.

## Attributes Reference

- `id` — идентификатор (совпадает с `name`).
- `role_id` (String) — Mongo id of the role. Pass this wherever an API wants a role identifier — `graylog_auth_backend.default_roles` in particular, which accepts a name without complaint and then breaks every login. `id` remains the role name for backward compatibility.
- `read_only` — флаг, показывающий, что роль системная и не может изменяться.

## Import

```
terraform import graylog_role.readonly tf-readonly
```
