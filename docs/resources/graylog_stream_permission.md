---
page_title: "graylog_stream_permission Resource"
subcategory: "Streams"
description: |-
  Manages role-based permissions (read/edit/share) for a specific Graylog Stream.
---

# graylog_stream_permission (Resource)

Ресурс для управления правами доступа к Stream через роль. Он добавляет/удаляет у роли только разрешения конкретного стрима, не затрагивая прочие права роли.

## Example Usage

```hcl
resource "graylog_stream_permission" "s_readwrite" {
  role_name = "Reader"
  stream_id = graylog_stream.s.id
  actions   = ["read", "edit"]
}
```

## Argument Reference

- `role_name` (Required, String) — имя роли, к которой применяются права.
- `stream_id` (Required, String) — ID стрима.
- `actions` (Optional, List(String)) — список действий: `read`, `edit`, `share`. По умолчанию `["read"]`.

## Attribute Reference

- `id` (String) — синтетический ID в формате `role_name/stream_id`.

## Import

Импорт по составному идентификатору `role_name/stream_id` (или `role_name:stream_id`):

```bash
terraform import graylog_stream_permission.s_readwrite Reader/5f1e7d1c2a3b4c5d6e7f8a9b
# или
terraform import graylog_stream_permission.s_readwrite Reader:5f1e7d1c2a3b4c5d6e7f8a9b
```

Примечания
- Ресурс управляет только правами конкретного `stream_id` у роли, оставляя остальные разрешения роли без изменений.
- Формат прав, выставляемых провайдером: `streams:<action>:<stream_id>`.
