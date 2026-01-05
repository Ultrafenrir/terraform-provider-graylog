###############################
# GELF TCP Input example
###############################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_input" "gelf_tcp" {
  title  = "gelf-tcp"
  type   = "org.graylog2.inputs.gelf.tcp.GELFTCPInput"
  global = true

  configuration = {
    bind_address    = "0.0.0.0"
    port            = 12201
    recv_buffer_size = 262144
    tls_enable      = false
  }
}
