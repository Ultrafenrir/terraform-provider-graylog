###############################
# Syslog UDP Input example
###############################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_input" "syslog_udp" {
  title  = "syslog-udp"
  type   = "org.graylog2.inputs.syslog.udp.SyslogUDPInput"
  global = true

  configuration = {
    bind_address = "0.0.0.0"
    port         = 5140
    recv_buffer_size = 262144
    allow_override_date = true
    charset = "UTF-8"
  }
}
