resource "graylog_index_set_field_type_profile" "http" {
  name        = "HTTP fields"
  description = "Consistent field types for application logs"

  custom_field_mappings = {
    "client.ip"   = "ip"
    response_code = "long"
    duration_ms   = "double"
    request_path  = "string"
    error_message = "string_fts"
  }
}

resource "graylog_index_set" "http" {
  title              = "HTTP logs"
  index_prefix       = "http_logs"
  shards             = 1
  replicas           = 0
  field_type_profile = graylog_index_set_field_type_profile.http.id

  rotation {
    class  = "org.graylog2.indexer.rotation.strategies.TimeBasedRotationStrategy"
    config = { rotation_period = "P1D" }
  }
  retention {
    class  = "org.graylog2.indexer.retention.strategies.DeletionRetentionStrategy"
    config = { max_number_of_indices = "30" }
  }
}

# Mapping/profile changes apply to future indices. Rotate separately in Graylog
# when you want a changed mapping to apply to newly ingested messages immediately.
# To detach: field_type_profile = ""
# To stop managing the binding while preserving it: remove field_type_profile.
