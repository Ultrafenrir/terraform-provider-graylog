############################################################
# Example: GELF UDP Input
############################################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_input" "gelf_udp" {
  title  = "gelf-udp"
  type   = "org.graylog2.inputs.gelf.udp.GELFUDPInput"
  global = true

  configuration = {
    bind_address     = "0.0.0.0"
    port             = 12201
    recv_buffer_size = 1048576
    tls_enable       = false
  }
}
