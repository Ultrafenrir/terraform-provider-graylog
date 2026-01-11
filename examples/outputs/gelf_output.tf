data "graylog_index_set_default" "this" {}

resource "graylog_stream" "s" {
  title        = "out-stream"
  description  = "Stream for outputs"
  index_set_id = data.graylog_index_set_default.this.id

  rule {
    field = "source"
    type  = 1
    value = "tf"
  }
}

resource "graylog_output" "gelf" {
  title = "to-local-gelf"
  type  = "org.graylog2.outputs.GelfOutput"
  configuration = jsonencode({
    hostname = "127.0.0.1"
    port     = 12201
  })
  streams = [graylog_stream.s.id]
}
