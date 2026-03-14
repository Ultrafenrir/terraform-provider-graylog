# Graylog Terraform Provider — Terraform Graylog automation

Production‑ready provider to automate Graylog operations with Terraform. Manage streams (and rules), inputs (and extractors), index sets, pipelines, dashboards, dashboard permissions (role‑based), alerts (Event Definitions) and more — as code. Works with Graylog v5, v6 and v7. For v6/v7 the `/api` prefix is handled automatically where required.

Search hints (SEO): Terraform Graylog provider, graylog terraform, terraform graylog, Graylog operation automation, graylog automation, terrafrom graylog provider, graylog teraform, terrafrom graylog.

## Installation (Terraform Registry)

Add the provider requirement (namespace is based on the GitHub org):

```hcl
terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = "~> 0.3"
    }
  }
}
```

Then configure the provider:

```hcl
# Basic username/password
provider "graylog" {
  url         = "http://localhost:9000/api"
  auth_method = "basic_userpass"
  username    = "admin"
  password    = "admin"
}

# API token
provider "graylog" {
  url         = "http://localhost:9000/api"
  auth_method = "basic_token"
  api_token   = var.graylog_api_token
}

# Bearer token
provider "graylog" {
  url          = "http://localhost:9000/api"
  auth_method  = "bearer"
  bearer_token = var.graylog_bearer
}

# Legacy base64 token (backwards compatibility only)
provider "graylog" {
  url   = "http://localhost:9000/api"
  token = base64encode("admin:admin")
}
```

See the `examples/` directory for standalone, copy‑pasteable snippets for each resource type.

## Why this provider (benefits)
- Terraform Graylog automation: declarative changes, code review, and drift detection for Graylog.
- Safe upgrades between Graylog 5 → 6 → 7 with a single Terraform state (covered by migration tests).
- Flexible resources: inputs with free‑form configuration and extractors; streams with rules; index sets and pipelines.
- Robust error handling with structured JSON parsing for clearer diagnostics from Graylog API.

### Provider configuration (auth, TLS/HTTP, OpenSearch, ENV)

- Auth methods (`auth_method`): `auto` (default), `basic_userpass`, `basic_token`, `bearer`, `basic_legacy_b64`.
  - `basic_userpass`: `username` + `password`.
  - `basic_token`: `api_token` (optionally `api_token_password`).
  - `bearer`: `bearer_token`.
  - `basic_legacy_b64`: legacy base64 `token` for compatibility.
- TLS/HTTP: `insecure_skip_verify`, `ca_bundle`, `client_cert`, `client_key`, `timeout`, `max_retries`, `retry_wait`.
- OpenSearch auxiliary settings (for features like snapshot repositories): `opensearch_url`, `opensearch_insecure`.
- Logging: `log_level` (tflog) controls client/provider verbosity.

Environment variables are supported for all provider options (used when attributes are unset):

`GRAYLOG_URL`, `GRAYLOG_AUTH_METHOD`, `GRAYLOG_USERNAME`, `GRAYLOG_PASSWORD`, `GRAYLOG_API_TOKEN`, `GRAYLOG_API_TOKEN_PASSWORD`, `GRAYLOG_BEARER_TOKEN`, `GRAYLOG_TOKEN`, `GRAYLOG_INSECURE` (1 to enable), `GRAYLOG_CA_BUNDLE`, `GRAYLOG_CLIENT_CERT`, `GRAYLOG_CLIENT_KEY`, `GRAYLOG_TIMEOUT`, `GRAYLOG_MAX_RETRIES`, `GRAYLOG_RETRY_WAIT`, `GRAYLOG_LOG_LEVEL`.

OpenSearch ENV (used when `opensearch_*` attributes are unset): `OPENSEARCH_URL`, `OPENSEARCH_INSECURE` (1 to enable).

## Supported Graylog versions

The provider is tested and supported against the following Graylog major versions:

- Graylog 5.x
- Graylog 6.x
- Graylog 7.x

CI runs integration, acceptance, and migration tests against all listed versions via docker‑compose to ensure compatibility.

## Examples

Examples are organized by resource type:
- examples/basic.tf — single file covering provider config and all main resources.
- examples/inputs/*.tf — inputs like Kafka, Syslog UDP, GELF TCP, Beats, Raw TCP/UDP, HTTP JSON. Each uses a flexible `configuration` map and may include `extractors`.
- examples/streams/*.tf — streams with multiple rules (integer `type` enum); includes `inverted` examples.
- examples/pipelines/*.tf — pipelines; `source` contains a full Graylog pipeline definition.
- examples/dashboards/*.tf — classic dashboards (title/description).
- examples/dashboards/permission.tf — role‑based permissions for classic dashboards via `graylog_dashboard_permission`.
- examples/alerts/*.tf — alert/Event Definition examples with pass‑through `config`.
 - examples/opensearch/snapshot_repo_fs.tf — OpenSearch filesystem snapshot repository.
 - examples/opensearch/snapshot_repo_s3.tf — OpenSearch S3‑compatible snapshot repository (MinIO in docker‑compose).

Notes:
- Stream rule `type` is an integer Graylog enum. Values vary by Graylog version (e.g., equals=1, regex=3). Consult your Graylog docs.
- Extractors are passed through as free‑form objects. Prefer a consistent format: either top‑level fields or a single `data` object with extractor payload.

### Canonical JSON and Import UX

- Canonical JSON serialization is applied to JSON‑like attributes to stabilize plans (avoid order‑only diffs). It is used for `graylog_event_notification.config`, `graylog_input.configuration`, and `graylog_input.extractors`.
- Import UX supports readable keys in addition to IDs:
  - `graylog_user` by `username`, `graylog_role` by `name`.
  - `graylog_stream`/`graylog_dashboard` by exact `title` or by ID (UUID/24‑hex). Use the explicit `title:` prefix to avoid ambiguity.
  See docs/index.md for detailed examples.

### Governance

- Stream permissions via `graylog_stream_permission` (actions: `read`, `edit`, `share`).
- Dashboard permissions via `graylog_dashboard_permission` (actions: `read`, `edit`, `share`; availability depends on Graylog version/image — classic dashboards).

### List Data Sources (V1)

For convenient lookups and iteration:

```hcl
data "graylog_streams" "all" {}
data "graylog_dashboards" "all" {}
data "graylog_index_sets" "all" {}
data "graylog_event_notifications" "all" {}

output "streams_map" {
  value = data.graylog_streams.all.title_map
}
```

See `docs/data-sources/` for each data source reference. New in this version:

- `graylog_ldap_group_members` — read‑only LDAP group listing for safe automation.

### Capability probe (skeleton)

The provider client includes a best‑effort capability probe (cached) to detect feature availability across Graylog versions/images (e.g., classic dashboard CRUD, event notifications). It is used internally for guarded tests/examples and can be expanded for future enterprise‑specific features.

## Docker Compose/dev environment

Use the provided `docker-compose.yml` to run a local Graylog for testing (also includes OpenSearch, MinIO and OpenLDAP for acceptance tests):

```bash
make graylog-up
make graylog-wait
# visit http://127.0.0.1:9000 (Graylog UI/API), http://127.0.0.1:9001 (MinIO console)
# MinIO S3 API is available at http://127.0.0.1:9002 (host port mapped to container 9000)
```

Authentication defaults in examples/tests assume `admin:admin`. You can override environment variables for integration tests:

```bash
URL=http://127.0.0.1:9000/api TOKEN=$(printf 'user:pass' | base64) make test-integration
```

Useful commands:
- `make graylog-up` — start the stack
- `make graylog-wait` — wait for API readiness
- `make graylog-logs` — follow Graylog service logs
- `make graylog-down` — stop and remove the stack

### Acceptance tests (optional, opt‑in via ENV)

Snapshot repositories against OpenSearch/MinIO:

```bash
ENABLE_OS_SNAPSHOT_ACC=1 go test -tags=acceptance ./internal/provider -run OpenSearchSnapshotRepository
```

Notes for snapshot repository acceptance:
- OpenSearch image in this repo includes the `repository-s3` plugin (installed in `compose/opensearch/Dockerfile`).
- Filesystem repositories require `path.repo` to include `/usr/share/opensearch/snapshots` (already set) and a bind‑mount from the host (`./compose/os_snapshots`).
- When using the local MinIO from docker‑compose, set S3 `endpoint = "http://127.0.0.1:9002"` in examples/tests.

LDAP group members against OpenLDAP:

```bash
ENABLE_LDAP_ACC=1 go test -tags=acceptance ./internal/provider -run LDAPGroupMembers
```

## Importing existing resources

You can import existing Graylog resources by ID or a human‑friendly key where supported:

```bash
# By ID (UUID or 24‑hex)
terraform import graylog_stream.s <stream_id>
terraform import graylog_dashboard.d <dashboard_id>
terraform import graylog_input.i <input_id>
terraform import graylog_pipeline.p <pipeline_id>
terraform import graylog_alert.a <event_definition_id>
terraform import graylog_index_set.is <index_set_id>

# By title (explicit prefix)
terraform import graylog_stream.s "title:errors"
terraform import graylog_dashboard.d "title:Ops overview"

# By username / role name
terraform import graylog_user.u alice
terraform import graylog_role.r read_only
```

After import, run `terraform plan` to see any drift and adjust your configuration.

## Integration tests

Integration tests run against a real Graylog via docker‑compose:
1. Requirements: Docker/Compose; ports 9000 (Graylog UI/API) and 9200 (OpenSearch) available.
2. Run once:
   ```bash
   make test-integration
   ```
   The target brings up Graylog, waits for readiness, then runs `go test` with the `integration` tag.

Note: Integration tests are marked with `//go:build integration` and are not executed by a regular `make test`.

## Migration tests (Terraform state 5→6→7)

To validate state migration across Graylog major versions (5 → 6 → 7) using a single Terraform state:

1. Prereqs: Docker/Compose; Terraform CLI; ports 9000/9200 available.
2. Run:
   ```bash
   make test-migration
   ```
   The target will:
   - Build the provider locally and configure Terraform dev overrides.
   - Start Graylog 5.x via docker-compose, apply `test/migration/step1` and ensure no drift.
   - In-place upgrade to 6.x (preserving volumes), apply `step2`, ensure no drift.
   - Upgrade to 7.x, apply `step3`, ensure no drift; optionally destroy at the end.

Notes:
- The migration covers all supported resources across the steps: inputs, streams (+rules), index sets, pipelines, dashboards, dashboard widgets, alerts (Event Definitions), event notifications, outputs, LDAP settings, roles, and users.
- Shared local backend is used: state file at `test/migration/shared/terraform.tfstate`.
- You can preserve resources after a successful run for debugging by setting `SKIP_DESTROY=1`.

## Releases and publishing

- GitHub Actions build and publish artifacts on tags matching `v*`.
- Artifacts include platform zips, `SHA256SUMS` and `SHA256SUMS.sig` signed with your GPG key.
- To publish a new version: push a tag, for example `git tag v0.1.0 && git push origin v0.1.0`.
