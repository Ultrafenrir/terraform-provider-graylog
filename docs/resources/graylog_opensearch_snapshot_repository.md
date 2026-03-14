---
page_title: "graylog_opensearch_snapshot_repository Resource - Graylog Terraform Provider"
subcategory: "OpenSearch & Backups"
description: |-
  Manages an OpenSearch Snapshot Repository (fs/s3 or any plugin type via generic settings). Use it to enable Graylog stream data backups via OpenSearch snapshots.
---

# graylog_opensearch_snapshot_repository

Manages an OpenSearch Snapshot Repository. Supports any repository `type` via a generic `settings` map. Additionally provides typed convenience blocks for `fs` and `s3` repositories.

Note for Graylog OSS users (Graylog stream backups / Graylog backups):

- Graylog OSS does not ship a built‑in "backup streams" feature. To back up your Streams’ data, use OpenSearch snapshots to back up the indices that Streams write to. This resource configures the snapshot repository (filesystem or S3) so you can trigger and automate snapshots on your cluster.
  - Keywords: graylog stream backups, graylog backups, graylog OpenSearch snapshots.

Note: This resource communicates directly with OpenSearch (not Graylog). Configure the provider with `opensearch_url` or `OPENSEARCH_URL` environment variable.

Prerequisites (when using the repo's docker-compose for local tests):
- OpenSearch has the `repository-s3` plugin installed (preinstalled in `compose/opensearch/Dockerfile`).
- MinIO S3 API is published on host port `9002` (console on `9001`).
- For filesystem repositories, OpenSearch lists the snapshot path in `path.repo` and the directory is bind-mounted (see `./compose/os_snapshots:/usr/share/opensearch/snapshots`).

## Example Usage

Filesystem repository (fs):

```hcl
provider "graylog" {
  url             = "http://127.0.0.1:9000"
  opensearch_url  = "http://127.0.0.1:9200"
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
```

S3-compatible repository (using MinIO in docker-compose):

```hcl
provider "graylog" {
  url            = "http://127.0.0.1:9000"
  opensearch_url = "http://127.0.0.1:9200"
}

resource "graylog_opensearch_snapshot_repository" "s3_repo" {
  name = "s3repo"
  type = "s3"

  s3_settings {
    bucket            = "tf-snapshots"
    endpoint          = "http://127.0.0.1:9002" # MinIO S3 API (mapped from container 9000 → host 9002)
    protocol          = "http"
    path_style_access = true
    access_key        = "minioadmin"
    secret_key        = "minioadmin"
    base_path         = "opensearch"
  }
}
```

Generic settings (any repository type):

```hcl
resource "graylog_opensearch_snapshot_repository" "custom" {
  name = "custom"
  type = "fs" # or other plugin type
  settings = {
    location = "/snapshots"
    compress = "true"
  }
}
```

Back up Streams data (what is actually backed up):

Streams in Graylog route messages into index sets. OpenSearch snapshots back up indices — therefore, by configuring a snapshot repository you enable backups of the data written by your Streams (their index sets). Creating the repository does not automatically take snapshots; you need to trigger snapshot creation (manually or via automation).

Example: trigger a snapshot for the indices of your default index set via OpenSearch API using a simple local-exec (replace the `indices` pattern with your index set prefix if needed):

```hcl
provider "graylog" {
  url            = "http://127.0.0.1:9000"
  opensearch_url = "http://127.0.0.1:9200"
}

resource "graylog_opensearch_snapshot_repository" "fs_repo" {
  name = "local"
  type = "fs"

  fs_settings {
    location = "/snapshots"
    compress = true
  }
}

# Demonstration-only: create a snapshot via OpenSearch HTTP API
resource "null_resource" "snapshot_now" {
  provisioner "local-exec" {
    command = <<EOT
      curl -sS -XPUT \
        -H 'Content-Type: application/json' \
        http://127.0.0.1:9200/_snapshot/${graylog_opensearch_snapshot_repository.fs_repo.name}/snap-$(date +%Y%m%d%H%M%S) \
        -d '{
              "indices": "graylog_*",           
              "ignore_unavailable": true,
              "include_global_state": false
            }'
    EOT
  }

  depends_on = [graylog_opensearch_snapshot_repository.fs_repo]
}
```

Tip: For production, prefer a scheduled job or a dedicated tool (e.g., OpenSearch plugins, Curator-equivalents, or CI) to create and rotate snapshots. Terraform is best used to provision the repository itself and to describe the desired backup targets/policies; not for periodic snapshot execution.

## FAQ / Notes

- Q: Does Graylog OSS have a native "Stream backup" button?
  - A: No. Implement "Graylog stream backups" by creating an OpenSearch snapshot repository and scheduling snapshots of your Graylog indices.
- Q: Which indices should I snapshot?
  - A: Snapshot index sets used by your Streams (e.g., `graylog_*` by default), plus any additional indices you rely on.

## Argument Reference

- `name` (Required) — Repository name.
- `type` (Required) — Repository type. Example: `fs`, `s3`.
- `settings` (Optional) — Generic key/value map of settings for the repository type. Use strings for values. Mutually exclusive with `fs_settings` and `s3_settings`.

### fs_settings
- `location` (Required) — Filesystem path (must be listed in `path.repo` on the OpenSearch node).
- `compress` (Optional) — Whether to compress snapshots.
- `max_snapshot_bytes_per_sec` (Optional) — Rate limit, e.g. `50mb`.
- `max_restore_bytes_per_sec` (Optional) — Rate limit for restore, e.g. `50mb`.
- `chunk_size` (Optional) — Chunk size, e.g. `10mb`.

### s3_settings
- `bucket` (Required) — Bucket name.
- `region` (Optional) — AWS region.
- `endpoint` (Optional) — Custom endpoint (e.g., MinIO).
- `base_path` (Optional) — Base path/prefix inside bucket.
- `protocol` (Optional) — `http` or `https`.
- `path_style_access` (Optional) — Force path-style requests (true for MinIO).
- `read_only` (Optional) — Open in read-only mode.
- `access_key`, `secret_key`, `session_token` (Optional, Sensitive) — Credentials (never returned in state).

## Import

Import by repository name:

```
terraform import graylog_opensearch_snapshot_repository.fs_repo local
```
