provider "graylog" {
  # Example assumes docker-compose from repo is running
  url            = "http://127.0.0.1:9000"
  opensearch_url = "http://127.0.0.1:9200"
}

resource "graylog_opensearch_snapshot_repository" "fs_repo" {
  name = "local"
  type = "fs"

  fs_settings {
    location                   = "/snapshots"
    compress                   = true
    max_snapshot_bytes_per_sec = "50mb"
    max_restore_bytes_per_sec  = "50mb"
  }
}
