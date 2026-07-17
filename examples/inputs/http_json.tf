############################################################
# Example: HTTP JSON Input
############################################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_input" "http_json" {
  title  = "http-json"
  type   = "org.graylog2.inputs.http.jsonpath.JsonPathInput" # or org.graylog2.inputs.http.json.JsonInput depending on version
  global = true

  configuration = jsonencode({
    bind_address = "0.0.0.0"
    port         = 18090
    # Depending on Graylog version & plugin the fields can differ
    recv_buffer_size = 1048576
    override_source  = "http"
    tls_enable       = false
  })

  # Example extractor, adjust to your plugin schema
  extractor {
    title          = "extract field foo"
    extractor_type = "json"
    source_field   = "message"
    target_field   = "foo"
    extractor_config = jsonencode({
      json_key = "foo"
    })
  }
}
