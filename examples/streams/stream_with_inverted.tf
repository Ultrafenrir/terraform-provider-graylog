############################################################
# Example: Stream with multiple rules and inverted
############################################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_index_set" "main" {
  title              = "main-index"
  description        = "Managed by Terraform"
  rotation_strategy  = "time"
  retention_strategy = "delete"
}

resource "graylog_stream" "filtered" {
  title        = "filtered-stream"
  description  = "Include WARN/ERROR but exclude healthchecks"
  index_set_id = graylog_index_set.main.id

  # type is an integer Graylog enum; values vary across versions
  rule {
    field = "level"
    type  = 1           # equals
    value = "ERROR"
  }

  rule {
    field = "level"
    type  = 1           # equals
    value = "WARN"
  }

  rule {
    field    = "message"
    type     = 3        # regex
    value    = ".*healthcheck.*"
    inverted = true     # exclude matches
  }

  # Note: matching_type is not exposed by the resource; default Graylog behavior applies.
}
