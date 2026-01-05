---
page_title: "graylog_index_set Resource - Graylog"
description: |-
  Manages a Graylog index set.
---

# graylog_index_set (Resource)

Manages a Graylog index set.

## Example Usage

```hcl
resource "graylog_index_set" "main" {
  title              = "main-index"
  description        = "Managed by Terraform"
  rotation_strategy  = "time"
  retention_strategy = "delete"
  shards             = 4
}
```

## Argument Reference

- `title` (String, Required) — Index set title.
- `description` (String, Optional) — Description.
- `rotation_strategy` (String, Optional) — Rotation strategy (e.g., `time`).
- `retention_strategy` (String, Optional) — Retention strategy (e.g., `delete`).
- `shards` (Number, Optional) — Number of shards.
- `timeouts` (Block, Optional) — Customize create/update/delete timeouts.

## Attributes Reference

- `id` — Index set ID.

## Import

```bash
terraform import graylog_index_set.is <index_set_id>
```
