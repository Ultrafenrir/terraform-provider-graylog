terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = ">= 0.1.0"
    }
  }
}

provider "graylog" {
  url   = var.graylog_url
  token = var.graylog_token
}

variable "graylog_url" { type = string }
variable "graylog_token" { type = string }
variable "alice_password" { type = string }

resource "graylog_user" "alice" {
  username = "alice"
  full_name = "Alice Doe"
  email = "alice@example.com"
  roles = ["Reader", "PowerUser"]
  timezone = "UTC"
  session_timeout_ms = 3600000
  disabled = false
  password = var.alice_password
}
