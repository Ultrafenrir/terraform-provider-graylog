# Changelog

## Unreleased

### Fixed
- **CRITICAL**: `graylog_index_set`: any change to a resource with an unconfigured `shards`/`replicas`/`index_analyzer`/`field_type_refresh_interval`/`index_optimization_max_num_segments`/`index_optimization_disabled`/`default` (i.e. left to Graylog's default) forced a full **destroy-and-recreate**, even for something as unrelated as changing `title` or `description`. Root cause: these are `Optional`+`Computed` attributes with no `UseStateForUnknown` plan modifier — Terraform's plan logic makes any Computed attribute without that modifier go Unknown as soon as *anything else* on the resource changes, and `shards`/`index_analyzer` had `RequiresReplace` (see below), turning that spurious Unknown into a full replace. Reported from a live v0.5.0 deployment as `shards = 4 -> 6 # forces replacement` and `index_analyzer = "standard" # forces replacement`, the latter followed by `Error: Error deleting index set — resource not found` on the next run (a stale/already-replaced ID getting deleted again). Added `UseStateForUnknown` to all of these attributes; this is now a plain in-place update.
- **Corrects a v0.5.0 regression**: v0.5.0 added `RequiresReplace` to `shards` and `index_analyzer`, reasoning they were "immutable after creation like `index_prefix`" — this was wrong. Unlike `index_prefix` (which determines physical index naming and genuinely can't change without recreating), `shards`/`index_analyzer` in a Graylog index set only affect indices Graylog rotates to *in the future*; they never require recreating an index set that already holds data. `shards` is now a normal, freely-updatable field (no `RequiresReplace`). Per explicit request, `index_analyzer` is now **Computed-only** (`Optional` removed) — this provider no longer allows setting or changing it at all; attempting to set it in config is rejected outright ("Invalid Configuration for Read-Only Attribute") rather than risking another destructive-replace footgun. `index_prefix` remains the *only* attribute that forces replacement — an index set holds data, and nothing except renaming should ever recreate it.
- `graylog_index_set`: `Delete` now treats "already not found" as success instead of an error, so a resource that's already gone (e.g. from a prior partial/erroneous replace cycle caused by the bug above) doesn't hard-fail the next `terraform apply`.
- Updated `TestAccIndexSet_immutableFieldsForceReplacement` and `TestAccIndexSet_update` to assert the corrected behavior (`plancheck.ExpectResourceAction(..., ResourceActionUpdate)` for `shards` changes, `ResourceActionReplace` only for `index_prefix`), and added a no-op re-plan step to catch any future regression of the Unknown-cascade bug.

### Tested
- Reproduced live against Graylog 6.0.14 (docker-compose): confirmed the exact reported sequence (`shards = 4 -> 6` no longer shows `forces replacement`; changing an unrelated field like `title` no longer cascades every other unconfigured attribute to `(known after apply)`/replace; `index_analyzer = "..."` in config is now rejected at plan time instead of being silently accepted then fought over later).

## v0.5.0 (2026-07-20)

### Breaking Changes
- **`graylog_input`**: the `extractors` attribute (a single JSON-encoded string) was replaced with repeatable `extractor` blocks with explicit fields (`title`, `extractor_type`, `source_field`, `target_field`, `cursor_strategy`, `extractor_config`, `condition_type`, `condition_value`, `order`, and nested `converter` blocks). Existing state is migrated automatically (schema v6 -> v7); no manual action needed. Update `.tf` configs using `extractors = jsonencode([...])` to the new block syntax — see `docs/resources/graylog_input.md`.

### Fixed
- **CRITICAL**: `graylog_stream`: `matching_type` (AND/OR) was completely unwired — absent from the schema/model, so it could never be set and the client always sent `"AND"`. Added as an `Optional`+`Computed` attribute wired through Create/Update/Read.
- **CRITICAL**: `graylog_stream`: `UpgradeState` had the wrong method signature and silently never implemented `resource.ResourceWithUpgradeState`, so Terraform never called it — schema version bumps could hard-fail `terraform plan` for any existing state. Rewritten with the correct interface; added a compile-time assertion to catch this class of regression.
- **CRITICAL**: `graylog_input`/extractors: extractors were stored as a full free-form JSON blob and always got overwritten with Graylog's full server-echoed object (including `id`, `creator_user_id`, `created_at`, `metrics`, ...) on every `Read`. Since the planned value (from config) never matched, `Update` ran on **every single apply** and unconditionally deleted and recreated **all** extractors, resetting IDs/metrics and creating a window with no extraction. Replaced with structured `extractor` blocks and identity-based reconciliation (only add/remove/replace what actually changed).
- **CRITICAL**: `graylog_input`/extractors: `Create`/`Update` returned early on a mid-loop extractor failure without ever calling `resp.State.Set`, which could orphan a successfully-created input (causing a duplicate on the next apply) or leave state pointing at extractors already deleted from Graylog. Both paths now persist whatever succeeded before surfacing the error.
- `CreateInputExtractor`/`DeleteInputExtractor` used a stale version-conditional API path that dropped the `/api` prefix for `APIV5` (the safe-default fallback version), while `ListInputExtractors` and the rest of the Input CRUD had already been unified to always use `/api/...`. This contradicted the project's own documented behavior (`/api` is required on all versions) and would 404 create/delete calls on any server resolved to `APIV5`.
- Same stale version-conditional `/api` path bug fixed in `GetLDAPSettings`/`UpdateLDAPSettings`.
- `graylog_input`: `global` now has a proper `Default` (`false`) instead of silently defaulting via an unchecked `ValueBool()` on a possibly-unknown value. (Reverted an initial attempt to also sync `global` back from the Create/Update API response — Graylog doesn't reliably echo it, which caused "Provider produced inconsistent result after apply" whenever `global = true` was set explicitly.)
- `graylog_input`: `node` was written back to state as an empty string instead of null when unset, causing a permanent diff on every plan for global inputs.
- **CRITICAL**: `graylog_stream`: `Create`/`Update` had the same partial-failure state-loss pattern as the input extractors bug above — a mid-loop stream-rule failure returned early without ever calling `resp.State.Set`, which could orphan a created stream or leave state referencing rules already deleted from Graylog. Rule creation is now best-effort (keeps going on a per-rule error) and state is always persisted with whatever succeeded.
- `graylog_stream`: `Update`'s rule-deletion loop silently discarded `DeleteStreamRule` errors (`_ = ...`) with no diagnostic at all; it now surfaces them as an error.
- `graylog_stream`: `description` and `index_set_id` were written back to state as empty strings instead of null when unset, causing a permanent diff on every plan (same root cause as the `graylog_input` `node` bug above).
- **CRITICAL**: `graylog_index_set`: `Create`/`Update` returned early on a failed post-create/post-update read-back without ever calling `resp.State.Set`, which could orphan a successfully-created index set (duplicate on next apply) or mask that an update had actually succeeded server-side. Both paths now persist the ID/plan before attempting the read-back.
- **CRITICAL**: `UpdateIndexSet` never propagated a changed `index_prefix` into the PUT body (it only ever resent whatever the pre-update GET returned), so changing `index_prefix` silently had no effect and produced a permanent, unresolvable diff. `index_prefix`, `shards`, and `index_analyzer` are genuinely immutable in Graylog after creation (already documented as such), so all three now have `RequiresReplace` plan modifiers instead of attempting a doomed in-place update.
- `UpdateIndexSet` could never clear `description` once set: a `!= ""` guard skipped writing it back, and separately the `IndexSet.Description` struct field's `omitempty` JSON tag dropped it from the request body even when the guard was removed. Both are fixed; clearing/omitting `description` now actually clears it.
- `graylog_index_set`: `applyIndexSetReadState` wrote back an empty server `description` as `StringValue("")` unconditionally, which broke the *other* half of the fix above — an unconfigured (null) `description` now produced "Provider produced inconsistent result after apply: .description: was null, but now cty.StringVal(\"\")". Fixed to preserve whichever "no description" representation (null vs `""`) the caller already had.
- **CRITICAL**: `graylog_input`: `Update` read `id` only from the plan. Since `id` is `Computed` (not `Optional`) with no `UseStateForUnknown` plan modifier, it is Unknown/empty in the plan during every Update — found via live testing against a real Graylog server, where every update sent `PUT /api/system/inputs/` with **no ID at all**, and Graylog rejected it with `405 Method Not Allowed`. This affected **every** `graylog_input` update, not just Kafka, on all versions (5.x/6.x/7.x); it went undetected because the only existing acceptance test never changed anything after create. Now reads `id` from prior state, matching the pattern already used correctly in `graylog_stream`/`graylog_index_set`.
- **CRITICAL**: `GetInput`/`ListInputs`: Graylog's response for these endpoints nests the configuration map under `"attributes"`, not `"configuration"` (confirmed live against Graylog 5.x/6.x/7.x) — only the create/update *request* body uses `"configuration"`. `Input.Configuration` was silently `nil` after every `Read`, meaning `configuration` drift was never actually detected/refreshed for **any** input type on **any** version. Added a fallback that reads `"attributes"` when `"configuration"` is absent/empty.
- `graylog_input`: `configuration` is now `Sensitive`. Graylog's own Kafka input flags its `custom_properties` field `is_sensitive` server-side (it commonly holds SSL keystore/truststore passwords and SASL JAAS config with an embedded password); since `Sensitive` in this framework only applies to a whole attribute, not sub-keys inside a JSON blob, the entire `configuration` string is now hidden in `terraform plan`/`apply` output.
- **Docs/examples**: `examples/inputs/kafka_raw.tf`, `examples/basic.tf`, and the Kafka section of `docs/resources/graylog_input.md` documented a config key set (`bootstrap_servers` as a list, `topics`, `security_protocol`, `ssl_truststore_location`, `sasl_mechanism`, etc.) that Graylog's native Kafka input (`org.graylog2.inputs.raw.kafka.RawKafkaInput`) has never supported on any version — none of those keys ever took effect, they were silently ignored by the backend. Verified the real schema live against Graylog 5.0.13, 6.0.14, and 7.0.10 (identical field set on all three) via `GET /api/system/inputs/types/org.graylog2.inputs.raw.kafka.RawKafkaInput`, and rewrote docs/examples around the actual fields: `legacy_mode` (must be set explicitly to `false` — its default is version-dependent: `true` on 5.x/6.x, `false` on 7.x — for `bootstrap_server` to take effect), `bootstrap_server` (single string, not a list), `topic_filter` (regex), `fetch_min_bytes`, `fetch_wait_max`, `threads`, `offset_reset`, `group_id`, and `custom_properties` (newline-separated `key=value` Kafka client properties — this is where SSL/SASL/keystore settings actually go). Also removed a fabricated input type, `org.graylog.plugins.kafka.input.KafkaJsonInput`, which doesn't exist in Graylog's registered input types on any tested version.
- `examples/basic.tf`: `graylog_alert`'s `config` attribute (a JSON-encoded string) was assigned a bare HCL object instead of `jsonencode({...})`, which fails Terraform's type check — same class of bug as the Kafka examples above, found while validating them.

### Tested
- The full `graylog_input` Kafka flow (create, update, `terraform plan` idempotency, `custom_properties` round-trip with SSL/SASL-style values) was verified end-to-end with real `terraform apply`/`plan`/`destroy` runs against live Graylog 5.0.13, 6.0.14, and 7.0.10 instances (docker-compose, `GRAYLOG_VERSION=5|6|7`) — not just unit tests.

### Notes
- `CreateDashboardWidget`/`EventNotification` functions in `internal/client/client.go` have a similar-looking version-conditional path pattern but include real, tested version-specific fallback logic (see comments) — left untouched pending separate verification.
- `graylog_index_set`'s `default` field is sent as a plain body field on create/update; whether that's sufficient to actually change Graylog's default index set (vs. requiring a dedicated endpoint) is unverified from code alone — see the resource docs.
- Compatible with Graylog 5.x, 6.x, and 7.x — this was specifically re-verified live for the `graylog_input`/Kafka fixes above (see "Tested"); the earlier stream/index_set fixes in this release were only verified against 6.0.14.

## v0.3.5 (2026-04-19)

### Breaking Changes
- **REMOVED**: Deprecated legacy fields `rotation_strategy` and `retention_strategy` from Index Set resource and data sources. These fields were never sent to Graylog API (marked with `json:"-"`) and only caused confusion and "unknown value" errors. Use `rotation` and `retention` blocks instead, which are fully supported since Graylog 5.x.

### Fixed
- **CRITICAL**: Index Set Update: fixed 405 errors caused by provider's Update method reading ID from Plan instead of State. Computed fields like `id` are not present in the Plan, resulting in empty ID being passed to UpdateIndexSet.
- **CRITICAL**: Index Set Create/Update: removed logic that was incorrectly nullifying `rotation` and `retention` blocks after apply. Now all fields are consistently preserved from API response via `applyIndexSetReadState`.
- **CRITICAL**: Index Set Update: fixed missing fields in IndexSet struct and implemented read-modify-write pattern. Added `writable`, `creation_date`, `can_be_default`, and `index_template_type` fields that are required by Graylog API.
- **CRITICAL**: IndexSet struct: removed `omitempty` from `replicas` and `index_optimization_disabled` fields - Graylog 7.x requires these fields to be present in all requests. This fixes 400 errors "Missing required properties: replicas indexOptimizationDisabled".
- **CRITICAL**: IndexSet struct: field `Writable` was not serialized to JSON (had `json:"-"` tag), causing incomplete PUT requests. Now properly serializes as `"writable"` field.
- **CRITICAL**: Stream Update: removed incorrect method fallbacks (PATCH/POST) that caused 405 errors. Now correctly uses only PUT method on `/api/streams/{id}` endpoint.
- **CRITICAL**: Index Set rotation/retention config: fixed "Provider produced inconsistent result" errors caused by Graylog API returning "type" field and numeric values in scientific notation (e.g., `2e+07`). Now filters out "type" field and formats floats without scientific notation.
- Index Set Update: implemented read-modify-write pattern - GET current state, merge changes, PUT complete object. Graylog API requires all fields in PUT requests.
- Simplified UpdateStream implementation - removed complex fallback chains that were masking real API errors.
- Simplified Create and Update methods - removed conditional logic for nullifying nested blocks, now all fields are consistently applied from API response.

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

## v0.3.3 (2026-04-15)
### Fixed
- Index Set: поддержка legacy поля `rotation_strategy` восстановлена для обратной совместимости (конвертация в block при чтении).

## v0.3.2 (2026-04-14)
### Changed
- Index Set (`graylog_index_set`):
  - API теперь требует блоки `rotation` / `retention` (Graylog 5+). Flat‑атрибуты `rotation_strategy_class` / `retention_strategy_class` теперь **computed** и опциональные (для чтения); при создании используйте блоки `rotation` и `retention`.
- Dashboard Widget: удалены deprecated типы виджетов (не работают в Graylog 6/7), оставлены современные (6+).

### Tests
- Интеграционные тесты для Graylog 5.x / 6.x / 7.x.
- `make test-migration` — миграция с версии на версию (окружение Docker Compose).

## v0.3.1 (2026-04-10)
### Fixed
- Pipeline: поддержка симлинка с модификацией статуса default на подробный объект.

## v0.3.0 (2026-04-08)
### Added
- Alert (Event Definition): ресурс `graylog_alert` для создания событий, привязки условий и нотификаций.
- Data Source `graylog_index_set_default`: получение default index set для использования в других ресурсах.

### Changed
- Graylog plugin framework обновлён до последних версий (совместимость с Terraform 1.5+).

## v0.2.8 (2026-03-25)
### Fixed
- Stream: исправлена вложенность правил stream rules, конвертация plain list → nested list, если требуется Graylog API.

## v0.2.7 (2026-03-20)
### Added
- Dashboard Widget: поддержка всех атрибутов (filters, stream filters, sort, grouping, etc.).

## v0.2.6 (2026-03-15)
### Changed
- Role LDAP Group Mapping: добавлена возможность указывать порядок сортировки при создании ресурса.

## v0.2.5 (2026-03-10)
### Fixed
- Input: поправлена сериализация атрибутов `global=true`.

## v0.2.4 (2026-03-05)
### Added
- Pipeline Rule: поддержка импорта по ID.

## v0.2.3 (2026-03-01)
### Changed
- Stream: убрано поле `disabled` (некорректно работало). Вместо него используйте `remove_matches_from_default_stream` для контроля потока сообщений.

## v0.2.2 (2026-02-25)
### Fixed
- Index Set: улучшена валидация параметра `field_type_refresh_interval`.

## v0.2.1 (2026-02-20)
### Added
- User: ресурс для управления пользователями Graylog (username, email, roles, permissions, etc.).

## v0.2.0 (2026-02-15)
### Added
- Dashboard: полная поддержка дашбордов Graylog.
- Dashboard Widget: создание и управление виджетами дашбордов.
- Event Notification: настройки уведомлений для событий/алертов.

## v0.1.5 (2026-02-10)
### Fixed
- Stream Rules: исправлена логика для правил с пустыми/null значениями.

## v0.1.4 (2026-02-05)
### Changed
- LDAP Settings: добавлена поддержка TLS конфигурации.

## v0.1.3 (2026-02-01)
### Fixed
- Pipeline: исправлено создание пайплайнов с множественными правилами.

## v0.1.2 (2026-01-25)
### Added
- Pipeline Connection: привязка стримов к пайплайнам.

## v0.1.1 (2026-01-20)
### Fixed
- Index Set: исправлена ошибка при чтении `retention_strategy`.

## v0.1.0 (2026-01-15)
### Initial Release
- Core resources: Index Set, Stream, Input, Pipeline, Pipeline Rule, Role, LDAP Group Mapping.
- Data sources: Index Set.
- Compatibility: Graylog 5.x, 6.x (частично 7.x).
