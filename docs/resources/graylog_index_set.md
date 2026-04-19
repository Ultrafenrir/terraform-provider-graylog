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
# Minimal configuration (defaults from Graylog)
resource "graylog_index_set" "minimal" {
  title        = "minimal-index"
  index_prefix = "minimal"
  shards       = 1
  replicas     = 0
  default      = false
}

# Full configuration with rotation and retention (Graylog 5+)
resource "graylog_index_set" "full" {
  title        = "logs"
  index_prefix = "logs"
  description  = "Managed by Terraform"

  rotation {
    class  = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
    config = {
      max_docs_per_index = "20000000"
    }
  }

  retention {
    class  = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = {
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
- `index_prefix` (String, Required) — Index name prefix (lowercase letters, numbers, dash, underscore).
- `shards` (Number, Optional) — Number of Elasticsearch shards (must be >= 0).
- `replicas` (Number, Optional) — Number of Elasticsearch replicas (must be >= 0).
- `index_analyzer` (String, Optional) — Elasticsearch analyzer to use (defaults to 'standard').
- `field_type_refresh_interval` (Number, Optional) — Field type refresh interval in milliseconds (defaults to 5000).
- `index_optimization_max_num_segments` (Number, Optional) — Max number of segments for index optimization (>=1, defaults to 1).
- `index_optimization_disabled` (Boolean, Optional) — Disable index optimization (defaults to false).
- `default` (Boolean, Optional) — Whether this is the default index set.
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
