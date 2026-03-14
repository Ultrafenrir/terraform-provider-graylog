# Changelog

## v0.4.0 (2026-03-14)
### Added
- Resource: `graylog_opensearch_snapshot_repository` — manage OpenSearch snapshot repositories (generic `type + settings`; typed blocks `fs_settings` and `s3_settings`).
- Data source: `graylog_ldap_group_members` — read members of an LDAP group (safe, read‑only) to drive user sync via `for_each`.
- Provider options: `opensearch_url`, `opensearch_insecure` (+ ENV: `OPENSEARCH_URL`, `OPENSEARCH_INSECURE`).
- Docker compose fixtures: MinIO (S3‑compatible, S3 API on host 9002) and OpenLDAP with LDIF (alice/bob, group `devops`) for acceptance tests.

### Tests
- Acceptance tests (tag `acceptance`):
  - OpenSearch snapshot repository (FS typed, FS generic, S3 against MinIO) — green on GL 5/6/7.
  - LDAP group members (expects group `devops` with 2 members) — green on GL 5/6/7.
  - Stream permissions via `graylog_stream_permission` (CRUD + import) — green on GL 5/6/7.

### Docs
- New docs for `graylog_opensearch_snapshot_repository` and `graylog_ldap_group_members` (with examples).
- README/docs: added notes about MinIO S3 API on port 9002, OpenSearch `repository-s3` plugin, and `path.repo` bind‑mount for FS snapshots.

## v0.3.0 (2026-02-28)
### Added
- Provider authentication methods: `auto`, `basic_userpass`, `basic_token` (with optional `api_token_password`), `bearer`, and legacy `basic_legacy_b64`.
- TLS/HTTP options: `insecure_skip_verify`, `ca_bundle`, `client_cert`, `client_key`, `timeout`, `max_retries`, `retry_wait`.
- Logging adapter to Terraform tflog (no secrets leakage in logs).
- Canonical JSON helper for deterministic serialization; integrated into:
  - `graylog_event_notification` (`config`)
  - `graylog_input` (`configuration` and `extractors`)
- Typed Event Definitions: 
  - typed `threshold {}` block for `graylog_alert` with runtime validations and fallback `config` escape‑hatch;
  - typed `aggregation {}` block (maps to `type = aggregation-v1`), mutual exclusivity with `threshold {}`;
  - new fields for typed blocks: `grace_ms`, `backlog_size`, and shorthand `filter { streams = [...] }` → mapped to payload `streams`.
- Import UX improvements:
  - `graylog_user` by `username` (fallback: id)
  - `graylog_role` by `name` (fallback: id)
  - `graylog_stream`/`graylog_dashboard` by `title:` or by ID (UUID/24-hex); ambiguous titles fail with an error
- New resources:
  - `graylog_stream_output_binding` — bind a Stream to an Output (CRUD + import)
  - `graylog_stream_permission` — role-based permissions for Streams (read/edit/share) (CRUD + import)
  - `graylog_dashboard_permission` — role-based permissions for Dashboards (read/edit/share) (CRUD + import)
- Client methods:
  - `ListStreams()` with robust response shape handling
  - `ListStreamOutputs()` to read stream-output bindings
  - Capability probe (skeleton) via `client.GetCapabilities()` with lazy probing and caching.
- New list data sources (V1):
  - `graylog_streams`, `graylog_dashboards`, `graylog_index_sets`, `graylog_event_notifications` — each returns `items` and a convenience `title_map`.
  - `graylog_inputs`, `graylog_users` — both return `items` and `title_map` (for users, map is `username -> id` with safe fallback on older versions).
 - Capability gating (version-aware validations): provider now probes feature availability and returns clear errors for unsupported features in resources (e.g., classic dashboards CRUD, event notifications).

### Tests
- Unit tests extended for canonical JSON and permission helpers.
- Integration tests green on Graylog 5/6/7 (known skips for classic Dashboard CRUD on some versions/images).
- Acceptance tests green on 5/6/7 (known skips for Dashboard/Dashboard Widget/Event Notification where image/version limits apply).
- Migration test `make test-migration` covers Terraform state across 5 → 6 → 7.

### Docs
- Provider configuration docs expanded (auth methods, TLS/HTTP, logging, ENV list).
- Canonical JSON and Import UX sections added with examples.
- Resource docs added for new resources; README updated with Governance section and examples.
- Typed alerts docs: `graylog_alert` documents both `threshold` and `aggregation` typed blocks with examples; added `grace_ms`, `backlog_size`, and `filter.streams`.
- List data sources docs: added pages for `graylog_streams`, `graylog_dashboards`, `graylog_index_sets`, `graylog_event_notifications`; index and README include quick examples.

### Notes
- Classic Dashboard CRUD/permissions may be unavailable on some GL 5.x/6.x images; tests and examples include guards and skips.

### Changed
- `graylog_stream` Update is now diff‑aware for rules: applies minimal add/remove operations instead of full resync, avoiding noisy diffs.
 - `graylog_stream_output_binding` now performs diff‑aware operations: no‑op if already attached; on Update attaches the new binding first and only then detaches the old one; Delete is a no‑op if already detached.

## v0.2.0
- Context propagation: client supports `WithContext(ctx)`; all provider resources and data sources pass Terraform operation context to API calls.
- Structured logging: client exposes `Logger` interface (default no-op) and logs requests, responses, retries, and errors with structured fields.
- Migration tests: Makefile target `make test-migration` runs Terraform state migration scenario across Graylog 5→6→7 using a shared local backend; covers all supported resources stepwise.

## v0.1.1
- Client: robust JSON error handling added. Graylog API error payloads are parsed into a structured `GraylogError` with status, message, validation details, and raw body. Retry logic remains intact.

## v0.1.0
Initial release of the Graylog Terraform Provider with the following capabilities:

- Provider targets Graylog v5, v6, and v7; API prefix `/api` is handled automatically for v6/v7.

- Resources
  - graylog_input — manage inputs of any type with flexible `configuration = map(dynamic)`; full support for Kafka inputs (all settings) and optional `extractors` (pass-through objects).
  - graylog_stream — manage streams with dedicated stream rules API; `rule` block supports `id`, `field`, `type` (int enum), `value`, `inverted`, `description`.
  - graylog_pipeline — classic pipelines with full `source` text; CRUD.
  - graylog_dashboard — classic dashboards (title/description); CRUD.
  - graylog_alert — Event Definitions with pass-through `config` map and `notification_ids`.
  - graylog_index_set — index sets with validation and migration of legacy rotation/retention fields.

- Data sources
  - graylog_index_set, graylog_stream, graylog_input.

- Validation (runtime diagnostics)
  - Required fields and cross-field checks (e.g., `global`/`node` for inputs), non-empty titles, rule types >= 0, soft warnings for long descriptions/empty lists, etc.

- Documentation and examples
  - Comprehensive docs under `docs/` rendered by Terraform Registry, including an extensive Kafka Raw input configuration guide.
  - Standalone examples under `examples/` covering inputs (Kafka, Syslog, GELF, Beats, Raw, HTTP JSON with extractors), streams with rules (incl. inverted), pipelines, dashboards, alerts, and data sources.

- CI/CD
  - GitHub Actions: CI workflow for lint/tests/build; Release workflow builds multi-OS/arch artifacts, generates `SHA256SUMS`, signs with GPG, runs integration tests across Graylog 5/6/7 prior to releasing.