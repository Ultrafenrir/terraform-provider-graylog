#########################################
# Stream with multiple rules (int types)
#########################################

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "admin-token"
}

resource "graylog_index_set" "main" {
  title        = "main-index"
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
