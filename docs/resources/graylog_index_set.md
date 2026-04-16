---
page_title: "graylog_index_set Resource - Graylog Terraform Provider"
subcategory: "Index Sets"
description: |-
  Terraform Graylog provider: manage Graylog index sets (automation/IaC). Useful keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# graylog_index_set (Resource)

Manages a Graylog index set. Part of the Graylog Terraform Provider for Graylog automation.

## Example Usage

```hcl
# Minimal (legacy flat fields)
resource "graylog_index_set" "main" {
  title              = "main-index"
  description        = "Managed by Terraform"
  rotation_strategy  = "time"
  retention_strategy = "delete"
  shards             = 4
}

# Recommended (Graylog 5+): explicit class + config blocks
resource "graylog_index_set" "with_delete" {
  title        = "logs"
  index_prefix = "logs"

  rotation {
    class  = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
    config = {
      max_docs_per_index = "20000000"
    }
  }

  retention {
    # Deletion strategy
    class  = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = {
      # Keep last N indices, older ones will be deleted
      max_number_of_indices = "20"
    }
  }
}
```

### Rotation strategy examples (Graylog 5+)

Below are standalone examples for all rotation types. Configure retention separately via the `retention` block if you want old indices to be deleted after a limit.

```hcl
# Rotate by number of documents (MessageCountRotationStrategy)
resource "graylog_index_set" "rotate_by_docs" {
  title        = "logs-by-docs"
  index_prefix = "logs-docs"

  rotation {
    class  = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
    config = {
      # Maximum messages per index before rotation
      max_docs_per_index = "20000000"
    }
  }
}

# Rotate by index size (SizeBasedRotationStrategy)
resource "graylog_index_set" "rotate_by_size" {
  title        = "logs-by-size"
  index_prefix = "logs-size"

  rotation {
    class  = "org.graylog2.indexer.rotation.strategies.SizeBasedRotationStrategy"
    config = {
      # Maximum size per index in bytes (example: 10 GiB)
      max_size = "10737418240"
    }
  }
}

# Rotate by time period (TimeBasedRotationStrategy)
resource "graylog_index_set" "rotate_by_time" {
  title        = "logs-by-time"
  index_prefix = "logs-time"

  rotation {
    class  = "org.graylog2.indexer.rotation.strategies.TimeBasedRotationStrategy"
    config = {
      # ISO-8601 period (e.g., P1D = 1 day, PT1H = 1 hour)
      rotation_period = "P1D"
    }
  }
}
```

## Argument Reference

- `title` (String, Required) — Index set title.
- `description` (String, Optional) — Description.
- `rotation_strategy` (String, Optional) — Legacy rotation strategy hint (e.g., `time`). Prefer the `rotation` block below on Graylog 5+.
- `retention_strategy` (String, Optional) — Legacy retention strategy hint (e.g., `delete`). Prefer the `retention` block below on Graylog 5+.
- `shards` (Number, Optional) — Number of shards.
- `timeouts` (Block, Optional) — Customize create/update/delete timeouts.

### rotation (Block) — Graylog 5+
- `class` (String, Optional) — Fully qualified rotation strategy class, e.g. `org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy`.
- `config` (Map(String), Optional) — Strategy config map. Use string values (numbers can be stringified):
  - For MessageCountRotationStrategy: `max_docs_per_index`
  - For SizeBasedRotationStrategy: `max_size`
  - For TimeBasedRotationStrategy: `rotation_period`

### retention (Block) — Graylog 5+
- `class` (String, Optional) — Fully qualified retention strategy class, e.g. `org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy`.
- `config` (Map(String), Optional) — Strategy config map. For deletion use:
  - `max_number_of_indices` — keep last N indices; older are deleted.

## Attributes Reference

- `id` — Index set ID.

## Import

```bash
terraform import graylog_index_set.is <index_set_id>
```
