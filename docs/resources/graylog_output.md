---
page_title: "graylog_output Resource - Graylog Terraform Provider"
subcategory: "Outputs"
description: |-
  Terraform Graylog provider: manage Graylog outputs and attach them to streams (automation/IaC). Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation. Управляет Output в Graylog и его привязками к Stream’ам.
---

# graylog_output

Ресурс создает/обновляет Output в Graylog и позволяет привязать его к одному или нескольким Stream. Входит в Graylog Terraform Provider для автоматизации операций с Graylog.

## Example Usage

```
data "graylog_index_set_default" "this" {}

resource "graylog_stream" "s" {
  title        = "out-stream"
  description  = "Stream for outputs"
  index_set_id = data.graylog_index_set_default.this.id
}

resource "graylog_output" "gelf" {
  title = "to-local-gelf"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
  streams = [graylog_stream.s.id]
}
```

## Argument Reference

- `title` — (Required) Название Output.
- `type` — (Required) Тип Output (FQCN класса в Graylog), например `org.graylog2.outputs.GelfOutput`.
- `configuration` — (Optional) Конфигурация Output в виде JSON-строки (`jsonencode({...})`).
- `streams` — (Optional) Список ID стримов, к которым нужно прикрепить этот Output.

## Attributes Reference

- `id` — идентификатор Output.

## Import

```
terraform import graylog_output.gelf <output_id>
```
