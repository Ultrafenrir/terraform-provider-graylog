############################################################
# Example: Beats Input
############################################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_input" "beats" {
  title  = "beats-input"
  type   = "org.graylog2.inputs.beats.BeatsInput"
  global = true

  configuration = {
    bind_address     = "0.0.0.0"
    port             = 5044
    recv_buffer_size = 1048576
    tls_enable       = false
  }

  # Optional extractors (free-form)
  extractors = []
}
