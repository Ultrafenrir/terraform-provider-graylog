terraform {
  required_providers {
    graylog = {
      source = "Ultrafenrir/graylog"
    }
  }
  backend "local" {
    path = "../shared/terraform.tfstate"
  }
}

variable "url" {
  type = string
}

variable "token" {
  type = string
  sensitive = true
}

variable "prefix" {
  description = "Unique prefix for names and index prefix to avoid collisions between runs"
  type        = string
}

provider "graylog" {
  url   = var.url
  token = var.token
}

# Index set (base)
resource "graylog_index_set" "is" {
  title              = "${var.prefix}-index"
  description        = "Terraform migration base index set"
  index_prefix       = var.prefix

  lifecycle {
    ignore_changes = [
      # GL5 returns legacy rotation/retention/shards defaults; ignore to avoid drift
      rotation_strategy,
      retention_strategy,
      shards,
      field_type_refresh_interval,
      index_analyzer,
      index_optimization_disabled,
      index_optimization_max_num_segments,
      replicas,
      retention,
      rotation,
    ]
  }
}

# Stream with optional link to index set
resource "graylog_stream" "s" {
  title        = "${var.prefix}-stream"
  description  = "Stream for migration tests"
  index_set_id = graylog_index_set.is.id
  disabled     = true

  lifecycle {
    ignore_changes = [
      # rule IDs are computed by API and may appear after first read
      rule,
      # link to index set may appear as computed during the first refresh
      index_set_id,
    ]
  }

  rule {
    field       = "source"
    type        = 1 # equals
    value       = "terraform"
    inverted    = false
    description = "match source"
  }
}

// Pipeline, Dashboard and Input are created from step2 (GL6+) to avoid GL5 API mismatches.

// Alert resource is created starting from step2 (GL6+). Skipped on GL5 due to API schema differences.

# Role
resource "graylog_role" "r" {
  name        = "${var.prefix}-role"
  description = "Role for migration tests"
  permissions = ["dashboards:read", "indices:read"]
}
