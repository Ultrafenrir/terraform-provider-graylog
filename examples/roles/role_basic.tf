resource "graylog_role" "readonly" {
  name        = "tf-readonly"
  description = "Readonly role managed by Terraform"
  permissions = [
    "dashboards:read",
    "indices:read",
  ]
}
