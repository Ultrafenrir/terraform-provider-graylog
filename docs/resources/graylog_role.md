---
page_title: "graylog_role Resource"
subcategory: "Security"
description: |-
  Управляет ролями в Graylog (название роли используется как идентификатор).
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
- `read_only` — флаг, показывающий, что роль системная и не может изменяться.

## Import

```
terraform import graylog_role.readonly tf-readonly
```
