#########################################
# Stream with multiple rules (int types)
#########################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_index_set" "main" {
  title              = "main-index"
  rotation_strategy  = "time"
  retention_strategy = "delete"
}

resource "graylog_stream" "errors_timeouts" {
  title        = "errors-and-timeouts"
  description  = "Only ERROR level and messages containing 'timeout'"
  index_set_id = graylog_index_set.main.id

  # type is Graylog enum (integer); typical values:
  # 1 = match exact; 3 = regex; values may differ by version
  rule {
    field = "level"
    type  = 1
    value = "ERROR"
  }

  rule {
    field = "message"
    type  = 3
    value = ".*timeout.*"
    description = "Contains 'timeout'"
  }
}
