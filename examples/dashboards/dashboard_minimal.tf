############################
# Minimal Dashboard example
############################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_dashboard" "ops" {
  title       = "Ops overview"
  description = "Classic dashboard"
}
