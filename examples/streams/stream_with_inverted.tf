############################################################
# Example: Stream with multiple rules and inverted
############################################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_index_set" "main" {
  title        = "main-index"
  description  = "Managed by Terraform"
  index_prefix = "main"
  shards       = 1
  replicas     = 1

  rotation {
    class = "org.graylog2.indexer.rotation.strategies.MessageCountRotationStrategy"
    config = {
      max_docs_per_index = "20000000"
    }
  }

  retention {
    class = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = {
      max_number_of_indices = "20"
    }
  }
}

resource "graylog_stream" "filtered" {
  title        = "filtered-stream"
  description  = "Include WARN/ERROR but exclude healthchecks"
  index_set_id = graylog_index_set.main.id

  # "OR": a message matches if it satisfies any rule (level == ERROR or level == WARN or
  # message doesn't match the healthcheck regex). Use "AND" (the default) to require all
  # rules to match instead.
  matching_type = "OR"

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
}
