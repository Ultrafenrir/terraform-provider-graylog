resource "graylog_opensearch_snapshot_repository" "s3_repo" {
  name = "s3repo"
  type = "s3"

  s3_settings {
    bucket            = "tf-snapshots"
    endpoint          = "http://127.0.0.1:9002"
    protocol          = "http"
    path_style_access = true
    access_key        = "minioadmin"
    secret_key        = "minioadmin"
    base_path         = "examples"
  }
}
