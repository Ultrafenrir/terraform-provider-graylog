provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-base64-token"
}

resource "graylog_ldap_setting" "this" {
  enabled        = false
  ldap_uri       = "ldap://ldap.example.org:389"
  search_base    = "dc=example,dc=org"
  search_pattern = "(uid={0})"
}
