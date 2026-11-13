resource "graylog_opensearch_snapshot_repository" "s3_repo" {
  name = "s3repo"
  type = "s3"

  s3_settings {
    bucket            = "tf-snapshots"
    region            = "us-east-1"
    endpoint          = "http://seaweedfs:8333"
    protocol          = "http"
    path_style_access = true
    access_key        = "snapshot-test"
    secret_key        = "snapshot-test-secret"
    base_path         = "examples"
  }
}
