############################################################
# Example: Raw UDP Input
############################################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_input" "raw_udp" {
  title  = "raw-udp"
  type   = "org.graylog2.inputs.raw.udp.RawUDPInput"
  global = true

  configuration = {
    bind_address     = "0.0.0.0"
    port             = 5556
    recv_buffer_size = 1048576
    max_message_size = 2097152
    allow_override_date = false
    charset          = "UTF-8"
  }
}
