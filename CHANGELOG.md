# Changelog

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
