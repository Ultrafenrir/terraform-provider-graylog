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

  configuration = {
    bind_address = "0.0.0.0"
    port         = 18090
    # Depending on Graylog version & plugin the fields can differ
    recv_buffer_size = 1048576
    override_source  = "http"
    tls_enable       = false
  }

  # Example extractor (free-form), adjust to your plugin schema
  extractors = [
    {
      type          = "json"
      title         = "extract field foo"
      source_field  = "message"
      target_field  = "foo"
      json_key      = "foo"
    }
  ]
}
