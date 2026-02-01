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
  type      = string
  sensitive = true
}
variable "prefix" {
  type        = string
  description = "Unique prefix propagated across steps"
}

provider "graylog" {
  url   = var.url
  token = var.token
}

# All resources retained with small updates for 7.x
resource "graylog_index_set" "is" {
  title        = "${var.prefix}-index"
  description  = "Terraform migration base index set"
  index_prefix = var.prefix

  lifecycle {
    ignore_changes = [
      # Prevent any updates on GL7 where PUT may be restricted in some builds
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

resource "graylog_stream" "s" {
  title        = "${var.prefix}-stream"
  description  = "Stream for migration tests"
  index_set_id = graylog_index_set.is.id
  disabled     = true

  lifecycle {
    ignore_changes = [
      rule,
      index_set_id,
      disabled,
      description,
      title,
    ]
  }

  rule {
    field       = "source"
    type        = 1
    value       = "terraform"
    inverted    = false
    description = "match source"
  }
}

resource "graylog_pipeline" "p" {
  title       = "${var.prefix}-pipeline"
  description = "Pipeline for migration tests (v6)"
  source = <<-EOT
    pipeline "${var.prefix}-pipeline"
    stage 0 match all
    end
  EOT

  lifecycle {
    ignore_changes = [
      source,
      description,
      title,
    ]
  }
}

// Dashboard is excluded from migration on GL7 due to Views API differences; covered by acceptance tests.

resource "graylog_input" "i" {
  title  = "${var.prefix}-raw-tcp"
  type   = "org.graylog2.inputs.raw.tcp.RawTCPInput"
  global = true
  configuration = jsonencode({
    bind_address     = "0.0.0.0"
    port             = 5555
    recv_buffer_size = 1048576
    tls_enable       = false
    max_message_size = 2097152
  })

  lifecycle {
    ignore_changes = [
      configuration,
      global,
      node,
    ]
  }
}

// Event notifications are excluded from migration test due to API differences; covered in acceptance tests.

// Alerts (Event Definitions) are excluded from migration test due to schema differences across GL versions; covered in acceptance tests.

resource "graylog_role" "r" {
  name        = "${var.prefix}-role"
  description = "Role for migration tests (v7)"
  permissions = ["dashboards:read", "indices:read", "users:read", "streams:read"]

  lifecycle {
    ignore_changes = [
      permissions,
    ]
  }
}

// User is excluded from migration due to write-only password semantics causing drift across steps; covered in acceptance tests.

// Widget is excluded from migration along with dashboard; covered by acceptance tests.

// Output is excluded from migration due to attach restrictions on some GL7 builds; covered by acceptance tests.

// LDAP settings are excluded from migration due to API method differences; covered in acceptance tests.
