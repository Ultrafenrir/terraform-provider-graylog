---
page_title: "graylog_stream_output_binding Resource"
subcategory: "Streams"
description: |-
  Manages an attachment (binding) of a Graylog Output to a Stream.
---

# graylog_stream_output_binding (Resource)

Binding resource that attaches an existing Output to an existing Stream. This is useful when вы хотите управлять связью независимо от ресурсов `graylog_stream` и `graylog_output`.

Diff-aware behavior:
- Create: выполняет attach только если связь ещё не существует (идемпотентно).
- Update: при изменении пары `stream_id`/`output_id` сначала прикрепляет новую связь, затем аккуратно удаляет старую — только если она существует (минимальные операции).
- Delete: выполняет detach только при наличии связи (no-op, если уже удалена вне Terraform).

## Example Usage

```hcl
resource "graylog_stream_output_binding" "example" {
  stream_id = graylog_stream.s.id
  output_id = graylog_output.o.id
}
```

## Argument Reference

- `stream_id` (Required, String) — ID потока (Stream), к которому прикрепляется Output.
- `output_id` (Required, String) — ID аутпута (Output), который прикрепляется к Stream.

## Attribute Reference

- `id` (String) — синтетический ID в формате `stream_id/output_id`.

## Import

Можно импортировать существующую привязку по составному идентификатору `stream_id/output_id` (или `stream_id:output_id`):

```bash
terraform import graylog_stream_output_binding.example <stream_id>/<output_id>
# или
terraform import graylog_stream_output_binding.example <stream_id>:<output_id>
```

Если указать только `id` одного из ресурсов — импорт завершится ошибкой. При удалении связи вне Terraform ресурс будет помечен как удалённый при следующем `terraform plan/apply`.
