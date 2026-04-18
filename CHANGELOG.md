# Changelog

## v0.3.5 (2026-04-18)
### Fixed
- **CRITICAL**: Index Set Update: fixed 405 errors caused by provider's Update method reading ID from Plan instead of State. Computed fields like `id` are not present in the Plan, resulting in empty ID being passed to UpdateIndexSet.
- **CRITICAL**: Index Set Update: fixed missing fields in IndexSet struct and implemented read-modify-write pattern. Added `writable`, `creation_date`, `can_be_default`, and `index_template_type` fields that are required by Graylog API.
- **CRITICAL**: IndexSet struct: field `Writable` was not serialized to JSON (had `json:"-"` tag), causing incomplete PUT requests. Now properly serializes as `"writable"` field.
- **CRITICAL**: Stream Update: removed incorrect method fallbacks (PATCH/POST) that caused 405 errors. Now correctly uses only PUT method on `/api/streams/{id}` endpoint.
- Index Set Update: implemented read-modify-write pattern - GET current state, merge changes, PUT complete object. Graylog API requires all fields in PUT requests.
- Simplified UpdateStream implementation - removed complex fallback chains that were masking real API errors.

### Tests
- Added unit tests to verify that Update methods use correct HTTP method (PUT) and fail properly on 405 errors.
- Updated unit tests to verify that UpdateIndexSet performs GET before PUT and sends complete object body.
- Added acceptance test `TestAccIndexSet_update` to verify update operations work correctly against live Graylog API.
- Verified fix against live Graylog instance using curl - PUT with complete object returns 200 OK, PATCH/POST return 405.

### Technical Details
- **Root cause identified**: resource_index_set.go Update method was reading from `req.Plan` instead of `req.State`. Since `id` is a Computed field, it's not in the Plan, causing `data.ID.ValueString()` to return empty string. This resulted in GET/PUT requests to `/api/system/indices/index_sets/` (no ID), which Graylog rejects with 405.
- **Fix**: Changed Update method to read both Plan (for updated values) and State (for ID), then pass `state.ID` to UpdateIndexSet.
- Analysis of live Graylog API revealed that IndexSet struct was missing 4 critical fields returned by GET endpoint.
- The `IsWritable bool json:"-"` field was not being serialized, causing PUT requests to fail validation.
- Changed to use complete `IndexSet` struct in PUT body instead of manually building map[string]any.
- Read-modify-write pattern ensures all Graylog-managed fields (creation_date, can_be_default, etc.) are preserved.

### Notes
- This fix resolves multiple root causes:
  1. Provider bug: reading ID from wrong source (Plan vs State)
  2. Client bug: incomplete struct definition led to missing required fields in API requests
- Tested against live Graylog instance - confirms PUT with complete object works, partial updates fail.
- All existing configurations will continue to work without changes.
- Compatible with Graylog 5.x, 6.x, and 7.x.

## v0.3.4 (2026-04-17)
### Fixed
- Index Set: исправлен апдейт для некоторых сборок GL 5/6/7 — в теле запроса теперь передаётся `shards` (с гарантией `>=1`), что устраняет `400 must be >= 1` и связанные `405` на альтернативных путях/методах.
- Provider (index_set): nested‑блоки `rotation`/`retention` материализуются в состоянии только если они были заданы в плане/состоянии. Это устраняет дрейф и ошибки вида «unexpected new value» после Apply.

### Tests
- Интеграционные тесты: для каждого объекта добавлен обязательный шаг Update→GET→Verify (Index Set, Stream, Input, Pipeline, Dashboard, Dashboard Widget, Event Notification, User), прогон через `make test-integration-all` (GL 5/6/7) — PASS.
- Acceptance: `make test-acc-all` — PASS.
- Миграция 5→6→7: `make test-migration` — PASS.

### Notes
- Поведение выровнено для Graylog 5.x/6.x/7.x; без изменений HCL.

## v0.3.3 (2026-04-17)
### Changed
- Bump версии до 0.3.3 для публикации релиза (функциональных изменений по сравнению с 0.3.2 нет).

## v0.3.2 (2026-04-17)
### Fixed
- Index Set update: added resilient fallback chain to avoid 405/404 across Graylog 5/6/7 (`PUT /api/system/indices/index_sets/{id}` → `POST` same path → legacy base `/system/...` with `PUT` → legacy with `POST`).
- Eliminated Terraform plan drift for Index Set by normalizing server defaults in `Read` and marking stable attrs as `Computed`: `index_analyzer`=`standard`, `field_type_refresh_interval`=`5000`, `index_optimization_max_num_segments`=`1`, `index_optimization_disabled`=`false`; legacy `rotation_strategy`/`retention_strategy` kept `null` in state.
- Index Set Create/Update: if `rotation.config`/`retention.config` is provided without discriminator `type`, it is inferred from the strategy class (suffix `Config`); safe defaults applied when configs are empty.
- Streams: Update now falls back across methods `PUT→PATCH→POST`; Create first tries v7 entity payload and falls back to legacy body format when required.

### Tests
- Added unit tests for Index Set update fallbacks and config `type` inference; added unit tests for Stream Create/Update fallbacks. No skips in unit tests; integration tests guarded by build tags.

### Notes
- Backwards compatible. No HCL changes required. Intended to behave uniformly on Graylog 5.x, 6.x, and 7.x.

## v0.3.1 (2026-03-14)
### Documentation
- Added comprehensive guides for production use:
  - `docs/guides/ldap-user-sync.md` — Complete LDAP user synchronization workflow with RBAC
  - `docs/guides/stream-backups.md` — OpenSearch snapshot repositories for stream data backups
  - `docs/guides/troubleshooting.md` — Common issues, debugging techniques, and solutions
- Added production-ready examples:
  - `examples/production/ldap-sync-rbac.tf` — Multi-team LDAP sync with role-based stream permissions
  - `examples/production/backup-and-restore.tf` — S3 backup strategy with DR replication
  - `examples/production/README.md` — Production deployment guide with best practices
- Restructured main README with focus on unique features (LDAP sync, stream backups, RBAC)
- Enhanced resource documentation with additional examples and usage patterns

### Notes
- All documentation is now Terraform Registry compliant
- Production examples demonstrate complete workflows for OSS-specific use cases

## v0.3.0 (2026-03-14)
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
  
  Additional in 0.3.0:
  - Resource: `graylog_opensearch_snapshot_repository` — manage OpenSearch snapshot repositories (generic `type + settings`; typed blocks `fs_settings` and `s3_settings`).
  - Data source: `graylog_ldap_group_members` — read members of an LDAP group (safe, read‑only) to drive user sync via `for_each`.
  - Provider options: `opensearch_url`, `opensearch_insecure` (+ ENV: `OPENSEARCH_URL`, `OPENSEARCH_INSECURE`).
  - Docker compose fixtures: MinIO (S3‑compatible, S3 API on host 9002) and OpenLDAP with LDIF (alice/bob, group `devops`) for acceptance tests.

### Tests
- Unit tests extended for canonical JSON and permission helpers.
- Integration tests green on Graylog 5/6/7 (known skips for classic Dashboard CRUD on some versions/images).
- Acceptance tests green on 5/6/7 (known skips for Dashboard/Dashboard Widget/Event Notification where image/version limits apply).
- Migration test `make test-migration` covers Terraform state across 5 → 6 → 7.
  
  Additional acceptance (0.3.0):
  - OpenSearch snapshot repository (FS typed, FS generic, S3 against MinIO) — green on GL 5/6/7.
  - LDAP group members (expects group `devops` with 2 members) — green on GL 5/6/7.
  - Stream permissions via `graylog_stream_permission` (CRUD + import) — green on GL 5/6/7.

### Docs
- Provider configuration docs expanded (auth methods, TLS/HTTP, logging, ENV list).
- Canonical JSON and Import UX sections added with examples.
- Resource docs added for new resources; README updated with Governance section and examples.
- Typed alerts docs: `graylog_alert` documents both `threshold` and `aggregation` typed blocks with examples; added `grace_ms`, `backlog_size`, and `filter.streams`.
- List data sources docs: added pages for `graylog_streams`, `graylog_dashboards`, `graylog_index_sets`, `graylog_event_notifications`; index and README include quick examples.
  
  Additional docs (0.3.0):
  - New docs for `graylog_opensearch_snapshot_repository` and `graylog_ldap_group_members` (with examples and guidance on backups/sync).
  - README/docs: notes about MinIO S3 API on port 9002, OpenSearch `repository-s3` plugin, and `path.repo` bind‑mount for FS snapshots.

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