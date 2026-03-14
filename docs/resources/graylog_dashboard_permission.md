---
page_title: "graylog_dashboard_permission Resource"
subcategory: "Dashboards"
description: |-
  Manages role-based permissions (read/edit/share) for a specific Graylog Dashboard.
---

# graylog_dashboard_permission (Resource)

Ресурс для управления правами доступа к Dashboard через роль. Он добавляет/удаляет у роли только разрешения конкретного дашборда, не затрагивая прочие права роли.

> Capability note: для создания/обновления требуется поддержка классических (legacy) dashboards CRUD/permissions. На образах/версиях без legacy‑дашбордов операции завершатся понятной ошибкой. См. таблицу совместимости в `docs/index.md`.

## Example Usage

```hcl
resource "graylog_dashboard_permission" "d_readwrite" {
  role_name    = "Reader"
  dashboard_id = graylog_dashboard.ops.id
  actions      = ["read", "edit"]
}
```

## Argument Reference

- `role_name` (Required, String) — имя роли, к которой применяются права.
- `dashboard_id` (Required, String) — ID дашборда.
- `actions` (Optional, List(String)) — список действий: `read`, `edit`, `share`. По умолчанию `["read"]`.

## Attribute Reference

- `id` (String) — синтетический ID в формате `role_name/dashboard_id`.

## Import

Импорт по составному идентификатору `role_name/dashboard_id` (или `role_name:dashboard_id`):

```bash
terraform import graylog_dashboard_permission.d_readwrite Reader/5f1e7d1c2a3b4c5d6e7f8a9b
# или
terraform import graylog_dashboard_permission.d_readwrite Reader:5f1e7d1c2a3b4c5d6e7f8a9b
```

Примечания
- Ресурс управляет только правами конкретного `dashboard_id` у роли, оставляя остальные разрешения роли без изменений.
- Формат прав, выставляемых провайдером: `dashboards:<action>:<dashboard_id>`.
