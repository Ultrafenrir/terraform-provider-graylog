############################################################
# Example: Raw TCP Input
############################################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_input" "raw_tcp" {
  title  = "raw-tcp"
  type   = "org.graylog2.inputs.raw.tcp.RawTCPInput"
  global = true

  configuration = {
    bind_address     = "0.0.0.0"
    port             = 5555
    recv_buffer_size = 1048576
    tls_enable       = false
    max_message_size = 2097152
  }
}
