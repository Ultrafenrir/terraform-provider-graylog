# Complete LDAP Sync with Role-Based Stream Permissions
# This example demonstrates:
# - Reading LDAP group members
# - Creating Graylog users automatically
# - Assigning roles based on LDAP groups
# - Granting stream-level permissions to roles

terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = "~> 0.3"
    }
  }
}

provider "graylog" {
  url      = var.graylog_url
  username = var.graylog_admin_user
  password = var.graylog_admin_password
}

# Variables
variable "graylog_url" {
  description = "Graylog API URL"
  default     = "https://graylog.example.com/api"
}

variable "graylog_admin_user" {
  description = "Graylog admin username"
  default     = "admin"
}

variable "graylog_admin_password" {
  description = "Graylog admin password"
  sensitive   = true
}

variable "ldap_url" {
  description = "LDAP server URL"
  default     = "ldap://ldap.example.com:389"
}

variable "ldap_bind_dn" {
  description = "LDAP bind DN"
  default     = "cn=readonly,dc=example,dc=com"
}

variable "ldap_bind_password" {
  description = "LDAP bind password"
  sensitive   = true
}

variable "ldap_base_dn" {
  description = "LDAP base DN"
  default     = "dc=example,dc=com"
}

# LDAP Group Configuration
# Map LDAP groups to Graylog roles and stream permissions
locals {
  ldap_groups = {
    devops = {
      group_name  = "devops"
      graylog_role = "DevOpsRole"
      description = "DevOps team members"
      base_permissions = [
        "dashboards:read",
        "inputs:read",
        "metrics:read",
        "streams:read",
      ]
      stream_permissions = {
        app_logs    = ["read", "edit"]
        app_metrics = ["read"]
      }
    }

    security = {
      group_name  = "security-team"
      graylog_role = "SecurityRole"
      description = "Security team members"
      base_permissions = [
        "dashboards:read",
        "event_definitions:read",
        "event_notifications:read",
        "streams:read",
      ]
      stream_permissions = {
        security_events = ["read", "edit", "share"]
        audit_logs      = ["read", "edit"]
      }
    }

    engineering = {
      group_name  = "engineering"
      graylog_role = "EngineeringRole"
      description = "Engineering team members"
      base_permissions = [
        "dashboards:read",
        "inputs:read",
        "streams:read",
        "pipelines:read",
      ]
      stream_permissions = {
        app_logs      = ["read"]
        app_metrics   = ["read"]
        debug_logs    = ["read", "edit"]
      }
    }
  }
}

# Read LDAP Group Members
data "graylog_ldap_group_members" "teams" {
  for_each = local.ldap_groups

  url           = var.ldap_url
  bind_dn       = var.ldap_bind_dn
  bind_password = var.ldap_bind_password
  base_dn       = var.ldap_base_dn
  group_name    = each.value.group_name
}

# Flatten all LDAP users (deduplicate by username)
locals {
  # Create a map: username -> list of roles from all groups
  user_role_map = merge([
    for team_key, team_config in local.ldap_groups : {
      for member in data.graylog_ldap_group_members.teams[team_key].members :
      member.username => {
        username     = member.username
        email        = member.email
        display_name = member.display_name
        roles        = []  # Will be populated below
      }
    }
  ]...)

  # Accumulate roles for each user (if user is in multiple groups)
  users_with_roles = {
    for username, user_data in local.user_role_map :
    username => merge(user_data, {
      roles = distinct(flatten([
        "Reader",  # All users get Reader role
        [
          for team_key, team_config in local.ldap_groups :
          team_config.graylog_role
          if contains([
            for m in data.graylog_ldap_group_members.teams[team_key].members : m.username
          ], username)
        ]
      ]))
    })
  }
}

# Create Graylog Roles
resource "graylog_role" "team_roles" {
  for_each = local.ldap_groups

  name        = each.value.graylog_role
  description = "${each.value.description} (managed by Terraform)"
  permissions = each.value.base_permissions
}

# Create Graylog Users from LDAP
resource "graylog_user" "ldap_users" {
  for_each = local.users_with_roles

  username  = each.value.username
  email     = each.value.email
  full_name = coalesce(each.value.display_name, each.value.username)

  # Users authenticate via LDAP; no password storage in Terraform
  set_password = false

  # Assign accumulated roles
  roles = each.value.roles

  # Optional settings
  timezone         = "UTC"
  session_timeout  = 3600000  # 1 hour
}

# Create Streams
resource "graylog_stream" "team_streams" {
  for_each = {
    app_logs = {
      title       = "Application Logs"
      description = "Production application logs"
      field       = "application"
      value       = "myapp"
    }
    app_metrics = {
      title       = "Application Metrics"
      description = "Application performance metrics"
      field       = "metric_type"
      value       = "application"
    }
    security_events = {
      title       = "Security Events"
      description = "Security-related events and alerts"
      field       = "event_type"
      value       = "security"
    }
    audit_logs = {
      title       = "Audit Logs"
      description = "System audit trail"
      field       = "log_type"
      value       = "audit"
    }
    debug_logs = {
      title       = "Debug Logs"
      description = "Debug-level application logs"
      field       = "log_level"
      value       = "DEBUG"
    }
  }

  title       = each.value.title
  description = each.value.description
  disabled    = false

  rule {
    field = each.value.field
    type  = 1  # Exact match
    value = each.value.value
  }
}

# Grant Stream Permissions to Roles
# DevOps team permissions
resource "graylog_stream_permission" "devops_app_logs" {
  role_name = graylog_role.team_roles["devops"].name
  stream_id = graylog_stream.team_streams["app_logs"].id
  actions   = local.ldap_groups.devops.stream_permissions.app_logs
}

resource "graylog_stream_permission" "devops_app_metrics" {
  role_name = graylog_role.team_roles["devops"].name
  stream_id = graylog_stream.team_streams["app_metrics"].id
  actions   = local.ldap_groups.devops.stream_permissions.app_metrics
}

# Security team permissions
resource "graylog_stream_permission" "security_events" {
  role_name = graylog_role.team_roles["security"].name
  stream_id = graylog_stream.team_streams["security_events"].id
  actions   = local.ldap_groups.security.stream_permissions.security_events
}

resource "graylog_stream_permission" "security_audit" {
  role_name = graylog_role.team_roles["security"].name
  stream_id = graylog_stream.team_streams["audit_logs"].id
  actions   = local.ldap_groups.security.stream_permissions.audit_logs
}

# Engineering team permissions
resource "graylog_stream_permission" "engineering_app_logs" {
  role_name = graylog_role.team_roles["engineering"].name
  stream_id = graylog_stream.team_streams["app_logs"].id
  actions   = local.ldap_groups.engineering.stream_permissions.app_logs
}

resource "graylog_stream_permission" "engineering_app_metrics" {
  role_name = graylog_role.team_roles["engineering"].name
  stream_id = graylog_stream.team_streams["app_metrics"].id
  actions   = local.ldap_groups.engineering.stream_permissions.app_metrics
}

resource "graylog_stream_permission" "engineering_debug" {
  role_name = graylog_role.team_roles["engineering"].name
  stream_id = graylog_stream.team_streams["debug_logs"].id
  actions   = local.ldap_groups.engineering.stream_permissions.debug_logs
}

# Outputs for verification
output "ldap_users_synced" {
  description = "List of users synced from LDAP"
  value       = keys(local.users_with_roles)
}

output "user_role_assignments" {
  description = "User to role mappings"
  value = {
    for username, user in local.users_with_roles :
    username => user.roles
  }
}

output "streams_created" {
  description = "Streams and their IDs"
  value = {
    for key, stream in graylog_stream.team_streams :
    key => stream.id
  }
}

output "role_permissions" {
  description = "Roles and their base permissions"
  value = {
    for key, role in graylog_role.team_roles :
    key => role.permissions
  }
}
