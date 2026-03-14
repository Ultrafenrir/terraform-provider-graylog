# Graylog Terraform Provider

**Production-ready Terraform provider for Graylog OSS** — automate log management infrastructure with unique features not available in other providers.

[![Terraform Registry](https://img.shields.io/badge/terraform-registry-blue)](https://registry.terraform.io/providers/Ultrafenrir/graylog)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Ultrafenrir/terraform-provider-graylog)](https://github.com/Ultrafenrir/terraform-provider-graylog)
[![Tests](https://github.com/Ultrafenrir/terraform-provider-graylog/workflows/CI/badge.svg)](https://github.com/Ultrafenrir/terraform-provider-graylog/actions)

## Why This Provider?

**Unique features for Graylog OSS** (not available in other providers):

✅ **LDAP User Sync** — Automate user provisioning from LDAP/AD groups
✅ **Stream Backups** — Configure OpenSearch snapshot repositories for data backup
✅ **Role-Based Permissions** — Granular stream/dashboard access control (RBAC)
✅ **Multi-Version Support** — Tested against Graylog 5.x, 6.x, and 7.x
✅ **State Migration** — Upgrade Graylog versions (5→6→7) without recreating resources

**Keywords:** graylog terraform, terraform graylog provider, graylog stream backups, graylog ldap integration, graylog ldap sync, graylog opensearch snapshots, graylog backups, sync ldap groups to graylog.

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

## Quick Start

```hcl
terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = "~> 0.3"
    }
  }
}

provider "graylog" {
  url      = "https://graylog.example.com/api"
  username = "admin"
  password = var.graylog_password
}

# Create a stream with routing rules
resource "graylog_stream" "app_logs" {
  title       = "Application Logs"
  description = "Production app logs"

  rule {
    field = "application"
    type  = 1  # exact match
    value = "myapp"
  }
}

# Grant read access to DevOps role
resource "graylog_stream_permission" "devops_access" {
  role_name = "DevOps"
  stream_id = graylog_stream.app_logs.id
  actions   = ["read", "edit"]
}
```

## Unique Features (Graylog OSS)

### 1. LDAP User Sync

**Problem:** Graylog OSS doesn't include automated LDAP user synchronization.

**Solution:** Use `graylog_ldap_group_members` data source to automate user provisioning:

```hcl
# Read LDAP group members
data "graylog_ldap_group_members" "devops" {
  url           = "ldap://ldap.example.com:389"
  bind_dn       = "cn=readonly,dc=example,dc=com"
  bind_password = var.ldap_password
  base_dn       = "dc=example,dc=com"
  group_name    = "devops"
}

# Auto-create Graylog users
resource "graylog_user" "ldap_users" {
  for_each = { for m in data.graylog_ldap_group_members.devops.members : m.username => m }

  username     = each.key
  email        = each.value.email
  set_password = false  # Authenticate via LDAP
  roles        = ["DevOpsRole"]
}
```

**📚 [Complete LDAP Sync Guide](docs/guides/ldap-user-sync.md)** | **[Production Example](examples/production/ldap-sync-rbac.tf)**

---

### 2. Stream Data Backups

**Problem:** Graylog OSS doesn't provide built-in stream data backup functionality.

**Solution:** Configure OpenSearch snapshot repositories for automated backups:

```hcl
provider "graylog" {
  url            = "https://graylog.example.com/api"
  opensearch_url = "https://opensearch.example.com:9200"
}

# S3 snapshot repository
resource "graylog_opensearch_snapshot_repository" "backups" {
  name = "production-backups"
  type = "s3"

  s3_settings {
    bucket    = "graylog-snapshots"
    region    = "us-east-1"
    base_path = "daily"
    compress  = true
  }
}
```

Then trigger snapshots via OpenSearch API or automation (cron/Lambda).

**📚 [Complete Backup Guide](docs/guides/stream-backups.md)** | **[Production Example](examples/production/backup-and-restore.tf)**

---

### 3. Role-Based Stream Permissions

Grant granular access control for streams to different teams:

```hcl
# DevOps team: read + edit
resource "graylog_stream_permission" "devops_app_logs" {
  role_name = "DevOpsRole"
  stream_id = graylog_stream.app_logs.id
  actions   = ["read", "edit"]
}

# Security team: read-only
resource "graylog_stream_permission" "security_app_logs" {
  role_name = "SecurityRole"
  stream_id = graylog_stream.security_events.id
  actions   = ["read", "edit", "share"]
}
```

Combine with LDAP sync for complete automated RBAC.

## Supported Resources & Data Sources

### Resources (15)
**Core Infrastructure:**
- `graylog_stream` — Streams with routing rules
- `graylog_input` — Inputs (Kafka, Syslog, GELF, Beats, etc.) with extractors
- `graylog_output` — Outputs (GELF, HTTP, etc.)
- `graylog_pipeline` — Processing pipelines
- `graylog_index_set` — Index set configuration
- `graylog_dashboard` — Classic dashboards
- `graylog_dashboard_widget` — Dashboard widgets

**Security & Governance:**
- `graylog_user` — User management
- `graylog_role` — Role management
- `graylog_ldap_setting` — LDAP configuration
- `graylog_stream_permission` — Stream RBAC ⭐
- `graylog_dashboard_permission` — Dashboard RBAC
- `graylog_stream_output_binding` — Stream-to-output bindings

**Alerts:**
- `graylog_alert` — Event Definitions (typed + config modes)
- `graylog_event_notification` — Notifications (email, HTTP, etc.)

**Backups:**
- `graylog_opensearch_snapshot_repository` — OpenSearch snapshot repos (FS/S3) ⭐

### Data Sources (13)
**Lookups:**
- `graylog_stream`, `graylog_input`, `graylog_dashboard`, `graylog_user`, `graylog_index_set`, `graylog_event_notification`

**Lists (pagination support):**
- `graylog_streams`, `graylog_dashboards`, `graylog_inputs`, `graylog_users`, `graylog_index_sets`, `graylog_event_notifications`, `graylog_views`

**LDAP Integration:**
- `graylog_ldap_group_members` — Read LDAP group members ⭐

---

##  Production Features

- ✅ **Multi-version support:** Tested on Graylog 5.x, 6.x, 7.x
- ✅ **State migration tests:** Upgrade 5→6→7 without recreation
- ✅ **Import by title/username:** Human-friendly imports
- ✅ **Canonical JSON:** No spurious diffs for JSON fields
- ✅ **Flexible auth:** Basic, token, bearer, legacy modes
- ✅ **TLS/mTLS support:** CA bundles, client certs
- ✅ **Retry logic:** Exponential backoff for API failures
- ✅ **Structured logging:** tflog integration (no secret leakage)

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

## Documentation

### 📖 [Provider Documentation (Terraform Registry)](https://registry.terraform.io/providers/Ultrafenrir/graylog/latest/docs)

### Guides

- **[LDAP User Sync Guide](docs/guides/ldap-user-sync.md)** — Complete workflow for syncing LDAP users with RBAC
- **[Stream Backups Guide](docs/guides/stream-backups.md)** — Configure OpenSearch snapshots for data backup
- **[Troubleshooting Guide](docs/guides/troubleshooting.md)** — Common issues and debugging

### Examples

**Basic Examples:** `examples/`
- `basic.tf` — Provider config and core resources
- `inputs/*.tf` — Kafka, Syslog, GELF, Beats, HTTP JSON
- `streams/*.tf` — Streams with routing rules
- `pipelines/*.tf` — Processing pipelines
- `dashboards/*.tf` — Dashboards and permissions
- `alerts/*.tf` — Event Definitions and notifications
- `opensearch/*.tf` — Snapshot repositories (FS/S3)
- `ldap/*.tf` — LDAP settings
- `users/*.tf`, `roles/*.tf`, `outputs/*.tf`

**Production Examples:** `examples/production/`
- **[ldap-sync-rbac.tf](examples/production/ldap-sync-rbac.tf)** — Multi-team LDAP sync with stream permissions
- **[backup-and-restore.tf](examples/production/backup-and-restore.tf)** — S3 backups with DR replication

**Note:** Stream rule `type` is an integer enum (1=exact, 3=regex, 5=contains). Values may vary by Graylog version — consult your Graylog documentation.

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
