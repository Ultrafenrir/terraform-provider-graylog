#############################################
# Dashboard permissions (role-based example) #
#############################################

terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = "~> 0.1"
    }
  }
}

provider "graylog" {
  url         = var.url
  auth_method = "basic_legacy_b64" # or basic_userpass/basic_token/bearer
  token       = var.token
}

variable "url" {
  type        = string
  description = "Graylog API URL (e.g. http://localhost:9000/api)"
}

variable "token" {
  type        = string
  sensitive   = true
  description = "Base64 of user:pass for legacy basic auth, or use other supported auth methods"
}

resource "graylog_role" "r" {
  name        = "tf-example-dashperm"
  description = "Example role for dashboard permissions"
}

resource "graylog_dashboard" "d" {
  title       = "tf-example-dashboard"
  description = "Example classic dashboard"
}

resource "graylog_dashboard_permission" "p" {
  role_name    = graylog_role.r.name
  dashboard_id = graylog_dashboard.d.id
  actions      = ["read", "edit"]
}

output "dashboard_id" {
  value = graylog_dashboard.d.id
}
