# Changelog

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